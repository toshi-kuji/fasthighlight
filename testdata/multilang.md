# タスク管理サービス「Tasku」開発ガイド

この文書は、タスク管理サービス「Tasku」の開発者向けのガイドである。サーバー、Webの画面、モバイルのアプリ、データの分析、運用の手順を、実際のコードとともに説明する。言語ごとに、よく使う書き方をまとめてある。

## 1. 全体の構成

Taskuは、次の部品でできている。

| 部品 | 言語 | 役割 |
| --- | --- | --- |
| APIサーバー | Go | タスクの作成、更新、検索 |
| 通知の配信 | Rust | プッシュ通知とメールの送信 |
| Webの画面 | TypeScript | ブラウザで動く画面 |
| Androidのアプリ | Kotlin | Android端末のアプリ |
| iOSのアプリ | Swift | iPhoneとiPadのアプリ |
| 分析 | Python、R | 利用状況の集計とレポート |
| 旧管理画面 | PHP | 社内向けの管理画面(移行中) |

> 新しい部品を足すときは、先に設計の文書を書き、レビューを受けること。

## 2. 開発環境の準備

まず、リポジトリを取得し、必要なツールを入れる。次のスクリプトは、macOSとLinuxの両方で動く。

```bash
#!/usr/bin/env bash
set -euo pipefail

REPO_URL="https://example.com/tasku/tasku.git"
WORKDIR="${HOME}/src/tasku"

if [[ ! -d "$WORKDIR" ]]; then
  git clone "$REPO_URL" "$WORKDIR"
fi
cd "$WORKDIR"

# 必要なツールの確認
for cmd in go cargo node python3 docker; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "error: $cmd が見つからない" >&2
    exit 1
  fi
done

# 依存の取得
go mod download
(cd web && npm ci)
python3 -m venv .venv && source .venv/bin/activate
pip install -r analytics/requirements.txt

cat <<EOF > .env.local
DATABASE_URL=postgres://tasku:tasku@localhost:5432/tasku
REDIS_URL=redis://localhost:6379/0
LOG_LEVEL=${LOG_LEVEL:-debug}
EOF

echo "準備ができた: $(date '+%Y-%m-%d %H:%M:%S')"
```

設定は、TOMLのファイルにまとめる。環境ごとの値は、環境変数で上書きできる。

```toml
# config/tasku.toml
title = "Tasku"
version = "2.4.0"

[server]
host = "0.0.0.0"
port = 8080
read_timeout = "5s"
write_timeout = "10s"
allowed_origins = ["https://tasku.example.com", "http://localhost:3000"]

[database]
url = "postgres://tasku:tasku@localhost:5432/tasku"
max_open_conns = 20
max_idle_conns = 5

[features]
recurring_tasks = true
ai_summary = false
beta_users = [1001, 1002, 1003]

[[notifications.channels]]
name = "email"
enabled = true
rate_limit_per_minute = 120

[[notifications.channels]]
name = "push"
enabled = true
rate_limit_per_minute = 600
```

ローカルでは、Docker Composeでデータベースなどを立ち上げる。

```yaml
version: "3.9"
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_USER: tasku
      POSTGRES_PASSWORD: tasku
      POSTGRES_DB: tasku
    ports:
      - "5432:5432"
    volumes:
      - db-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U tasku"]
      interval: 5s
      retries: 10
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
  api:
    build: .
    depends_on:
      db:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://tasku:tasku@db:5432/tasku
      LOG_LEVEL: info
volumes:
  db-data: {}
```

## 3. データベース

タスクと利用者の表は、次のとおり。タスクの検索を速くするため、期限と状態に索引を張る。

```sql
CREATE TABLE users (
    id          BIGSERIAL PRIMARY KEY,
    email       VARCHAR(255) NOT NULL UNIQUE,
    name        VARCHAR(100) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE TABLE tasks (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       VARCHAR(200) NOT NULL,
    note        TEXT,
    status      VARCHAR(20) NOT NULL DEFAULT 'todo',
    priority    SMALLINT NOT NULL DEFAULT 2,
    due_at      TIMESTAMP WITH TIME ZONE,
    done_at     TIMESTAMP WITH TIME ZONE,
    CHECK (status IN ('todo', 'doing', 'done'))
);

CREATE INDEX idx_tasks_user_due ON tasks (user_id, due_at);
CREATE INDEX idx_tasks_status ON tasks (status) WHERE status <> 'done';

-- 期限切れのタスクを、利用者ごとに数える
SELECT u.email,
       COUNT(*) AS overdue,
       MIN(t.due_at) AS oldest_due
FROM tasks AS t
JOIN users AS u ON u.id = t.user_id
WHERE t.status <> 'done'
  AND t.due_at < now()
GROUP BY u.email
HAVING COUNT(*) >= 3
ORDER BY overdue DESC
LIMIT 50;
```

## 4. APIサーバー(Go)

APIサーバーは、標準ライブラリの`net/http`で書く。タスクの一覧を返すハンドラは、次のとおり。

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

// Task は 1 件のタスク。
type Task struct {
	ID       int64      `json:"id"`
	Title    string     `json:"title"`
	Status   string     `json:"status"`
	Priority int        `json:"priority"`
	DueAt    *time.Time `json:"due_at,omitempty"`
}

type Store interface {
	ListTasks(ctx context.Context, userID int64, limit int) ([]Task, error)
}

var ErrNotFound = errors.New("not found")

func ListHandler(s Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := strconv.ParseInt(r.URL.Query().Get("user"), 10, 64)
		if err != nil {
			http.Error(w, "bad user id", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		tasks, err := s.ListTasks(ctx, userID, 100)
		switch {
		case errors.Is(err, ErrNotFound):
			http.NotFound(w, r)
			return
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"tasks": tasks, "count": len(tasks)})
	}
}
```

APIの応答は、次の形のJSONになる。

```json
{
  "tasks": [
    {
      "id": 42,
      "title": "週次の報告を書く",
      "status": "doing",
      "priority": 1,
      "due_at": "2026-11-06T18:00:00+09:00"
    },
    {
      "id": 43,
      "title": "請求書を確認する",
      "status": "todo",
      "priority": 2
    }
  ],
  "count": 2,
  "next_cursor": null,
  "elapsed_ms": 3.25
}
```

## 5. 通知の配信(Rust)

通知の配信は、送信の失敗に強くするため、Rustで書いた。失敗したら、間隔を伸ばしながら再送する。

```rust
use std::collections::HashMap;
use std::time::Duration;

#[derive(Debug, Clone, PartialEq)]
pub enum Channel {
    Email(String),
    Push { device_token: String },
}

#[derive(Debug)]
pub struct Notification<'a> {
    pub user_id: u64,
    pub title: &'a str,
    pub body: String,
    pub channel: Channel,
}

pub trait Sender {
    fn send(&self, n: &Notification) -> Result<(), String>;
}

/// 失敗したら、間隔を 2 倍にしながら最大 `max_retries` 回まで送り直す。
pub fn send_with_retry<S: Sender>(s: &S, n: &Notification, max_retries: u32) -> Result<u32, String> {
    let mut wait = Duration::from_millis(100);
    for attempt in 0..=max_retries {
        match s.send(n) {
            Ok(()) => return Ok(attempt),
            Err(e) if attempt < max_retries => {
                eprintln!("retry {} for user {}: {}", attempt + 1, n.user_id, e);
                std::thread::sleep(wait);
                wait *= 2;
            }
            Err(e) => return Err(e),
        }
    }
    unreachable!()
}

pub fn count_by_channel(ns: &[Notification]) -> HashMap<&'static str, usize> {
    let mut m = HashMap::new();
    for n in ns {
        let key = match n.channel {
            Channel::Email(_) => "email",
            Channel::Push { .. } => "push",
        };
        *m.entry(key).or_insert(0) += 1;
    }
    m
}
```

## 6. Webの画面

### 6.1 TypeScript

画面の状態は、型を付けて管理する。APIの呼び出しは、失敗したときの型も含めて表す。

```typescript
export type Status = "todo" | "doing" | "done";

export interface Task {
  id: number;
  title: string;
  status: Status;
  priority: 1 | 2 | 3;
  dueAt?: string;
}

type Result<T> = { ok: true; value: T } | { ok: false; error: string };

export async function fetchTasks(userId: number, signal?: AbortSignal): Promise<Result<Task[]>> {
  try {
    const res = await fetch(`/api/tasks?user=${userId}`, { signal });
    if (!res.ok) {
      return { ok: false, error: `HTTP ${res.status}` };
    }
    const body = (await res.json()) as { tasks: Task[] };
    return { ok: true, value: body.tasks };
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : String(e) };
  }
}

export function groupByStatus(tasks: readonly Task[]): Record<Status, Task[]> {
  const groups: Record<Status, Task[]> = { todo: [], doing: [], done: [] };
  for (const t of tasks) {
    groups[t.status].push(t);
  }
  return groups;
}
```

### 6.2 JavaScript

ビルドの設定は、JavaScriptで書く。

```javascript
// vite.config.js
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const isProd = process.env.NODE_ENV === "production";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      "/api": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
  build: {
    sourcemap: !isProd,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes("node_modules")) {
            return "vendor";
          }
        },
      },
    },
  },
  define: {
    __BUILD_TIME__: JSON.stringify(new Date().toISOString()),
  },
});
```

### 6.3 HTMLとCSS

画面の枠は、次のHTMLで作る。

```html
<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Tasku</title>
  <link rel="stylesheet" href="/assets/app.css">
</head>
<body>
  <header class="app-header">
    <h1>Tasku</h1>
    <nav>
      <a href="/today" class="active">今日</a>
      <a href="/upcoming">これから</a>
    </nav>
  </header>
  <main id="app" data-user-id="1001">
    <!-- ここに画面を描く -->
    <p class="loading">読み込み中&hellip;</p>
  </main>
  <script type="module" src="/assets/main.js"></script>
</body>
</html>
```

色と余白は、CSSの変数でそろえる。

```css
:root {
  --color-primary: #2563eb;
  --color-done: #16a34a;
  --radius: 8px;
  --gap: 12px;
}

.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--gap) calc(var(--gap) * 2);
  border-bottom: 1px solid rgba(0, 0, 0, 0.08);
}

.task-card {
  border-radius: var(--radius);
  padding: 10px 14px;
  margin-bottom: var(--gap);
  background: #fff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.12);
  transition: transform 0.15s ease-in-out;
}

.task-card:hover {
  transform: translateY(-2px);
}

.task-card.done .title {
  color: var(--color-done);
  text-decoration: line-through;
}

@media (max-width: 600px) {
  .app-header nav {
    display: none;
  }
}
```

## 7. モバイルのアプリ

### 7.1 Kotlin(Android)

```kotlin
package com.example.tasku

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

data class Task(
    val id: Long,
    val title: String,
    val done: Boolean = false,
    val priority: Int = 2,
)

sealed interface UiState {
    object Loading : UiState
    data class Loaded(val tasks: List<Task>) : UiState
    data class Failed(val message: String) : UiState
}

class TaskViewModel(private val repo: TaskRepository) : ViewModel() {
    private val _state = MutableStateFlow<UiState>(UiState.Loading)
    val state: StateFlow<UiState> = _state

    fun load(userId: Long) {
        viewModelScope.launch {
            _state.value = try {
                val tasks = repo.list(userId).sortedBy { it.priority }
                UiState.Loaded(tasks)
            } catch (e: Exception) {
                UiState.Failed(e.message ?: "unknown error")
            }
        }
    }

    fun toggle(task: Task) {
        val current = (_state.value as? UiState.Loaded)?.tasks ?: return
        _state.value = UiState.Loaded(current.map { if (it.id == task.id) it.copy(done = !it.done) else it })
    }
}
```

### 7.2 Swift(iOS)

```swift
import SwiftUI

struct Task: Identifiable, Codable, Hashable {
    let id: Int
    var title: String
    var done: Bool = false
    var priority: Int = 2
}

@MainActor
final class TaskStore: ObservableObject {
    @Published private(set) var tasks: [Task] = []
    @Published var errorMessage: String?

    private let baseURL = URL(string: "https://tasku.example.com/api")!

    func load(userID: Int) async {
        do {
            let url = baseURL.appendingPathComponent("tasks")
            let (data, _) = try await URLSession.shared.data(from: url)
            tasks = try JSONDecoder().decode([Task].self, from: data)
                .sorted { $0.priority < $1.priority }
        } catch {
            errorMessage = "読み込みに失敗した: \(error.localizedDescription)"
        }
    }
}

struct TaskListView: View {
    @StateObject private var store = TaskStore()

    var body: some View {
        List(store.tasks) { task in
            HStack {
                Image(systemName: task.done ? "checkmark.circle.fill" : "circle")
                Text(task.title)
            }
        }
        .task { await store.load(userID: 1001) }
    }
}
```

## 8. 共通のライブラリ

### 8.1 C

日付の計算は、古い組み込み機器でも使えるよう、Cで書いた共通の部品を使う。

```c
#include <stdio.h>
#include <stdbool.h>
#include <stdint.h>

/* 閏年なら true を返す */
static bool is_leap(int32_t year) {
    return (year % 4 == 0 && year % 100 != 0) || year % 400 == 0;
}

static const uint8_t days_in_month[12] = {31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31};

/* 年の初めからの日数(1 月 1 日が 1) */
int day_of_year(int32_t year, int month, int day) {
    if (month < 1 || month > 12) {
        return -1;
    }
    int n = day;
    for (int m = 0; m < month - 1; m++) {
        n += days_in_month[m];
    }
    if (month > 2 && is_leap(year)) {
        n++;
    }
    return n;
}

int main(void) {
    printf("%d\n", day_of_year(2026, 11, 1));
    return 0;
}
```

### 8.2 C++

検索の索引は、C++で書いた。

```cpp
#include <algorithm>
#include <string>
#include <unordered_map>
#include <vector>

namespace tasku {

class Index {
public:
    void add(int id, const std::string& text) {
        for (auto& word : split(text)) {
            postings_[word].push_back(id);
        }
    }

    std::vector<int> search(const std::string& word) const {
        auto it = postings_.find(word);
        if (it == postings_.end()) {
            return {};
        }
        auto ids = it->second;
        std::sort(ids.begin(), ids.end());
        ids.erase(std::unique(ids.begin(), ids.end()), ids.end());
        return ids;
    }

private:
    static std::vector<std::string> split(const std::string& s) {
        std::vector<std::string> out;
        std::string cur;
        for (char c : s) {
            if (c == ' ') {
                if (!cur.empty()) out.push_back(std::move(cur));
                cur.clear();
            } else {
                cur += c;
            }
        }
        if (!cur.empty()) out.push_back(cur);
        return out;
    }

    std::unordered_map<std::string, std::vector<int>> postings_;
};

}  // namespace tasku
```

### 8.3 C#

社内の集計ツールは、C#で書いた。

```csharp
using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;

namespace Tasku.Reports
{
    public record TaskRow(long Id, string Title, string Status, DateTime? DueAt);

    public sealed class WeeklyReport
    {
        private readonly ITaskSource _source;

        public WeeklyReport(ITaskSource source) => _source = source;

        public async Task<string> BuildAsync(DateTime weekStart)
        {
            var rows = await _source.LoadAsync(weekStart, weekStart.AddDays(7));
            var done = rows.Count(r => r.Status == "done");
            var overdue = rows
                .Where(r => r.Status != "done" && r.DueAt is { } due && due < DateTime.Now)
                .OrderBy(r => r.DueAt)
                .ToList();

            var lines = new List<string>
            {
                $"週の始め: {weekStart:yyyy-MM-dd}",
                $"完了: {done} 件 / 全 {rows.Count} 件",
            };
            lines.AddRange(overdue.Select(r => $"- 期限切れ: {r.Title} ({r.DueAt:MM/dd})"));
            return string.Join(Environment.NewLine, lines);
        }
    }
}
```

### 8.4 Java

決済の連携は、Javaで書いた古い部品が残っている。

```java
package com.example.tasku.billing;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.List;
import java.util.Objects;

public final class Invoice {
    private static final BigDecimal TAX_RATE = new BigDecimal("0.10");

    private final String customerId;
    private final List<LineItem> items;

    public Invoice(String customerId, List<LineItem> items) {
        this.customerId = Objects.requireNonNull(customerId);
        this.items = List.copyOf(items);
    }

    public BigDecimal subtotal() {
        return items.stream()
                .map(LineItem::amount)
                .reduce(BigDecimal.ZERO, BigDecimal::add);
    }

    public BigDecimal total() {
        BigDecimal tax = subtotal().multiply(TAX_RATE).setScale(0, RoundingMode.DOWN);
        return subtotal().add(tax);
    }

    @Override
    public String toString() {
        return "Invoice{customer=" + customerId + ", total=" + total() + "}";
    }

    public record LineItem(String name, int quantity, BigDecimal unitPrice) {
        public BigDecimal amount() {
            return unitPrice.multiply(BigDecimal.valueOf(quantity));
        }
    }
}
```

## 9. 分析

### 9.1 Python

利用状況の集計は、Pythonで行う。

```python
from __future__ import annotations

import csv
from collections import Counter
from dataclasses import dataclass
from datetime import date, datetime
from pathlib import Path


@dataclass(frozen=True)
class Event:
    user_id: int
    kind: str
    at: datetime


def load_events(path: Path) -> list[Event]:
    """CSV のイベントの記録を読む。"""
    with path.open(encoding="utf-8") as f:
        return [
            Event(int(row["user_id"]), row["kind"], datetime.fromisoformat(row["at"]))
            for row in csv.DictReader(f)
        ]


def daily_active_users(events: list[Event]) -> dict[date, int]:
    users: dict[date, set[int]] = {}
    for e in events:
        users.setdefault(e.at.date(), set()).add(e.user_id)
    return {d: len(u) for d, u in sorted(users.items())}


if __name__ == "__main__":
    events = load_events(Path("events.csv"))
    kinds = Counter(e.kind for e in events)
    print(f"events: {len(events):,}")
    for kind, n in kinds.most_common(5):
        print(f"  {kind:<12} {n:>8}")
    for d, n in daily_active_users(events).items():
        print(d.isoformat(), n)
```

### 9.2 R

月ごとのレポートの図は、Rで作る。

```r
library(dplyr)
library(ggplot2)

events <- read.csv("events.csv", stringsAsFactors = FALSE)
events$at <- as.POSIXct(events$at, format = "%Y-%m-%dT%H:%M:%S")

monthly <- events %>%
  mutate(month = format(at, "%Y-%m")) %>%
  group_by(month, kind) %>%
  summarise(n = n(), users = n_distinct(user_id), .groups = "drop")

# 完了の割合
done_rate <- function(df) {
  total <- sum(df$n[df$kind == "created"])
  if (total == 0) return(NA_real_)
  sum(df$n[df$kind == "completed"]) / total
}

rates <- sapply(split(monthly, monthly$month), done_rate)
print(round(rates * 100, 1))

p <- ggplot(monthly, aes(x = month, y = users, fill = kind)) +
  geom_col(position = "dodge") +
  labs(title = "月ごとの利用者数", x = NULL, y = "利用者") +
  theme_minimal()
ggsave("monthly.png", p, width = 8, height = 4.5, dpi = 150)
```

## 10. 旧管理画面(PHP)

旧管理画面は、PHPで書かれている。新しい画面に移すまでは、ここを直す。

```php
<?php
declare(strict_types=1);

namespace Tasku\Admin;

use PDO;

final class UserController
{
    public function __construct(private PDO $db) {}

    public function index(array $query): string
    {
        $page = max(1, (int)($query['page'] ?? 1));
        $limit = 20;
        $stmt = $this->db->prepare(
            'SELECT id, email, name FROM users ORDER BY id DESC LIMIT :limit OFFSET :offset'
        );
        $stmt->bindValue(':limit', $limit, PDO::PARAM_INT);
        $stmt->bindValue(':offset', ($page - 1) * $limit, PDO::PARAM_INT);
        $stmt->execute();

        $rows = '';
        foreach ($stmt->fetchAll(PDO::FETCH_ASSOC) as $user) {
            $name = htmlspecialchars($user['name'], ENT_QUOTES, 'UTF-8');
            $rows .= "<tr><td>{$user['id']}</td><td>{$name}</td></tr>\n";
        }
        return "<table>{$rows}</table>";
    }
}
```

## 11. 運用

### 11.1 Ruby

定期の作業は、Rubyのスクリプトで行う。

```ruby
require "json"
require "net/http"
require "time"

module Tasku
  class Reminder
    DEFAULT_HOURS = 24

    def initialize(api_url:, token:, hours: DEFAULT_HOURS)
      @api_url = URI(api_url)
      @token = token
      @hours = hours
    end

    def due_soon
      res = Net::HTTP.get_response(@api_url + "/tasks?due_within=#{@hours}h")
      raise "HTTP #{res.code}" unless res.is_a?(Net::HTTPSuccess)

      JSON.parse(res.body, symbolize_names: true)[:tasks]
        .reject { |t| t[:status] == "done" }
        .sort_by { |t| Time.parse(t[:due_at]) }
    end

    def run
      tasks = due_soon
      return puts("no tasks") if tasks.empty?

      tasks.each do |t|
        puts format("%-30s %s", t[:title], t[:due_at])
      end
    end
  end
end

Tasku::Reminder.new(api_url: ENV.fetch("TASKU_API"), token: ENV["TASKU_TOKEN"]).run if $PROGRAM_NAME == __FILE__
```

### 11.2 設定の変更

設定を変えるときは、変更の差分をレビューに出す。

```diff
diff --git a/config/tasku.toml b/config/tasku.toml
index 3f2a1b0..8c9d4e2 100644
--- a/config/tasku.toml
+++ b/config/tasku.toml
@@ -9,8 +9,8 @@ allowed_origins = ["https://tasku.example.com", "http://localhost:3000"]
 [database]
 url = "postgres://tasku:tasku@localhost:5432/tasku"
-max_open_conns = 20
-max_idle_conns = 5
+max_open_conns = 40
+max_idle_conns = 10
 
 [features]
 recurring_tasks = true
-ai_summary = false
+ai_summary = true
```

### 11.3 外部の連携

外部の会計サービスとは、XMLでデータをやり取りする。

```xml
<?xml version="1.0" encoding="UTF-8"?>
<invoices xmlns="https://accounting.example.com/schema/v2" generated="2026-11-01T09:00:00+09:00">
  <invoice id="INV-2026-1101" currency="JPY">
    <customer code="C-1001">株式会社サンプル</customer>
    <!-- 明細 -->
    <items>
      <item sku="PLAN-PRO" quantity="12" unit-price="1200"/>
      <item sku="SEAT-ADD" quantity="5" unit-price="300"/>
    </items>
    <note><![CDATA[支払いは月末締め、翌月末払い。]]></note>
    <total tax-included="true">17490</total>
  </invoice>
</invoices>
```

## 12. 文書の書き方

この開発ガイドのような文書は、Markdownで書く。書き方の例は次のとおり。

```markdown
---
title: リリースノート 2.4.0
date: 2026-11-01
---

# リリースノート 2.4.0

## 新しい機能

- **繰り返しのタスク**: 毎週や毎月のタスクを作れるようにした
- [x] 通知の時刻を選べるようにした
- [ ] 共有のリスト(次の版で入れる)

1. 設定の画面を開く
2. 「通知」を選ぶ

> 古い版のアプリでは、繰り返しのタスクは表示されない。

詳しくは [ヘルプ](https://tasku.example.com/help) を見ること。`tasku sync` で手動で同期できる。
```

コマンドの出力の例のように、色付けしないものは`text`にする。

```text
$ tasku sync --verbose
connecting to https://tasku.example.com ... ok
downloaded 128 tasks (3 updated, 1 deleted)
done in 0.42s
```

以上で、開発ガイドを終える。分からないことがあれば、開発のチャンネルで質問すること。
