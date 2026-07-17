use std::{
    env,
    io::{Read, Write},
    net::TcpStream,
    path::Path,
    sync::{Arc, Mutex},
    time::Duration,
};

use axum::{
    Json, Router,
    extract::State,
    http::{HeaderMap, StatusCode, header},
    response::{Html, IntoResponse, Response},
    routing::{get, post},
};
use rusqlite::{Connection, OptionalExtension, params};
use serde::Serialize;

const MIN_VALUE: i64 = -1_000_000;
const MAX_VALUE: i64 = 1_000_000;

#[derive(Clone)]
struct AppState {
    database: Arc<Mutex<Connection>>,
    allowed_origins: Vec<String>,
}

#[derive(Debug, Serialize)]
struct CounterState {
    value: i64,
    updated_at: i64,
}

#[derive(Debug)]
enum AppError {
    Forbidden,
    Database(String),
}

impl IntoResponse for AppError {
    fn into_response(self) -> Response {
        let (status, code) = match self {
            Self::Forbidden => (StatusCode::FORBIDDEN, "origin_not_allowed"),
            Self::Database(ref message) => {
                eprintln!("database error: {message}");
                (StatusCode::INTERNAL_SERVER_ERROR, "internal_error")
            }
        };
        (status, Json(serde_json::json!({ "error": code }))).into_response()
    }
}

#[tokio::main]
async fn main() {
    let command = env::args().nth(1).unwrap_or_else(|| "serve".to_owned());
    let result = match command.as_str() {
        "serve" => serve().await,
        "healthcheck" => healthcheck(),
        other => Err(format!("unknown command {other:?}")),
    };
    if let Err(error) = result {
        eprintln!("{error}");
        std::process::exit(1);
    }
}

async fn serve() -> Result<(), String> {
    let listen_address = env_or_default("DEMO_COUNTER_LISTEN_ADDRESS", "0.0.0.0:8080");
    let database_path = env_or_default("DEMO_COUNTER_DATABASE_PATH", "/data/counter.db");
    let allowed_origins =
        parse_origins(&env::var("DEMO_COUNTER_ALLOWED_ORIGINS").unwrap_or_default());

    let database = open_database(Path::new(&database_path)).map_err(|error| error.to_string())?;
    let state = AppState {
        database: Arc::new(Mutex::new(database)),
        allowed_origins,
    };
    let app = Router::new()
        .route("/", get(page))
        .route("/assets/app.css", get(css))
        .route("/assets/app.js", get(javascript))
        .route("/api/state", get(get_state))
        .route("/api/increment", post(increment))
        .route("/api/decrement", post(decrement))
        .route("/api/reset", post(reset))
        .route("/health/live", get(liveness))
        .with_state(state);

    let listener = tokio::net::TcpListener::bind(&listen_address)
        .await
        .map_err(|error| error.to_string())?;
    println!("demo counter listening on {listen_address}");
    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal())
        .await
        .map_err(|error| error.to_string())
}

async fn shutdown_signal() {
    #[cfg(unix)]
    {
        use tokio::signal::unix::{SignalKind, signal};
        let mut terminate = signal(SignalKind::terminate()).expect("install SIGTERM handler");
        tokio::select! {
            _ = tokio::signal::ctrl_c() => {},
            _ = terminate.recv() => {},
        }
    }
    #[cfg(not(unix))]
    tokio::signal::ctrl_c()
        .await
        .expect("install signal handler");
}

async fn page() -> impl IntoResponse {
    ([(header::CACHE_CONTROL, "no-store")], Html(PAGE_HTML))
}

async fn css() -> impl IntoResponse {
    (
        [
            (header::CONTENT_TYPE, "text/css; charset=utf-8"),
            (header::CACHE_CONTROL, "public, max-age=3600"),
        ],
        APP_CSS,
    )
}

async fn javascript() -> impl IntoResponse {
    (
        [
            (header::CONTENT_TYPE, "text/javascript; charset=utf-8"),
            (header::CACHE_CONTROL, "public, max-age=3600"),
        ],
        APP_JS,
    )
}

async fn get_state(State(state): State<AppState>) -> Result<Json<CounterState>, AppError> {
    read_state(&state).map(Json)
}

async fn increment(
    State(state): State<AppState>,
    headers: HeaderMap,
) -> Result<Json<CounterState>, AppError> {
    require_origin(&state, &headers)?;
    change_value(&state, 1).map(Json)
}

async fn decrement(
    State(state): State<AppState>,
    headers: HeaderMap,
) -> Result<Json<CounterState>, AppError> {
    require_origin(&state, &headers)?;
    change_value(&state, -1).map(Json)
}

async fn reset(
    State(state): State<AppState>,
    headers: HeaderMap,
) -> Result<Json<CounterState>, AppError> {
    require_origin(&state, &headers)?;
    set_value(&state, 0).map(Json)
}

async fn liveness() -> impl IntoResponse {
    Json(serde_json::json!({ "status": "ok" }))
}

fn open_database(path: &Path) -> rusqlite::Result<Connection> {
    let connection = Connection::open(path)?;
    connection.busy_timeout(Duration::from_secs(2))?;
    connection.execute_batch(
        "PRAGMA journal_mode = WAL;
         PRAGMA synchronous = NORMAL;
         CREATE TABLE IF NOT EXISTS counter (
             id INTEGER PRIMARY KEY CHECK (id = 1),
             value INTEGER NOT NULL CHECK (value BETWEEN -1000000 AND 1000000),
             updated_at INTEGER NOT NULL
         );
         INSERT OR IGNORE INTO counter (id, value, updated_at)
         VALUES (1, 0, unixepoch());",
    )?;
    Ok(connection)
}

fn read_state(state: &AppState) -> Result<CounterState, AppError> {
    let database = state
        .database
        .lock()
        .map_err(|_| AppError::Database("database lock poisoned".to_owned()))?;
    query_state(&database).map_err(database_error)
}

fn change_value(state: &AppState, delta: i64) -> Result<CounterState, AppError> {
    let mut database = state
        .database
        .lock()
        .map_err(|_| AppError::Database("database lock poisoned".to_owned()))?;
    apply_delta(&mut database, delta).map_err(database_error)
}

fn set_value(state: &AppState, value: i64) -> Result<CounterState, AppError> {
    let database = state
        .database
        .lock()
        .map_err(|_| AppError::Database("database lock poisoned".to_owned()))?;
    database
        .execute(
            "UPDATE counter SET value = ?1, updated_at = unixepoch() WHERE id = 1",
            params![value.clamp(MIN_VALUE, MAX_VALUE)],
        )
        .map_err(database_error)?;
    query_state(&database).map_err(database_error)
}

fn apply_delta(database: &mut Connection, delta: i64) -> rusqlite::Result<CounterState> {
    let transaction = database.transaction()?;
    let current: i64 =
        transaction.query_row("SELECT value FROM counter WHERE id = 1", [], |row| {
            row.get(0)
        })?;
    let next = current.saturating_add(delta).clamp(MIN_VALUE, MAX_VALUE);
    transaction.execute(
        "UPDATE counter SET value = ?1, updated_at = unixepoch() WHERE id = 1",
        params![next],
    )?;
    transaction.commit()?;
    query_state(database)
}

fn query_state(database: &Connection) -> rusqlite::Result<CounterState> {
    database
        .query_row(
            "SELECT value, updated_at FROM counter WHERE id = 1",
            [],
            |row| {
                Ok(CounterState {
                    value: row.get(0)?,
                    updated_at: row.get(1)?,
                })
            },
        )
        .optional()?
        .ok_or(rusqlite::Error::QueryReturnedNoRows)
}

fn require_origin(state: &AppState, headers: &HeaderMap) -> Result<(), AppError> {
    let supplied = headers
        .get(header::ORIGIN)
        .and_then(|value| value.to_str().ok());
    if supplied.is_some_and(|origin| {
        state
            .allowed_origins
            .iter()
            .any(|allowed| allowed == origin)
    }) {
        Ok(())
    } else {
        Err(AppError::Forbidden)
    }
}

fn parse_origins(raw: &str) -> Vec<String> {
    raw.split(',')
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_owned)
        .collect()
}

fn database_error(error: rusqlite::Error) -> AppError {
    AppError::Database(error.to_string())
}

fn env_or_default(key: &str, fallback: &str) -> String {
    env::var(key)
        .ok()
        .map(|value| value.trim().to_owned())
        .filter(|value| !value.is_empty())
        .unwrap_or_else(|| fallback.to_owned())
}

fn healthcheck() -> Result<(), String> {
    let mut stream = TcpStream::connect_timeout(
        &"127.0.0.1:8080"
            .parse()
            .map_err(|error| format!("{error}"))?,
        Duration::from_secs(2),
    )
    .map_err(|error| error.to_string())?;
    stream
        .set_read_timeout(Some(Duration::from_secs(2)))
        .map_err(|error| error.to_string())?;
    stream
        .write_all(b"GET /health/live HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
        .map_err(|error| error.to_string())?;
    let mut response = [0_u8; 64];
    let count = stream
        .read(&mut response)
        .map_err(|error| error.to_string())?;
    let head = String::from_utf8_lossy(&response[..count]);
    if head.starts_with("HTTP/1.1 200") {
        Ok(())
    } else {
        Err(format!("unexpected health response: {head}"))
    }
}

const PAGE_HTML: &str = r#"<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>共享计数器 · HomeHub</title>
  <link rel="stylesheet" href="/demo/counter/assets/app.css">
</head>
<body>
  <main>
    <a class="back" href="/">← 返回 HomeHub</a>
    <section class="card">
      <p class="eyebrow">RUST · SQLITE 持久化服务</p>
      <h1>共享计数器</h1>
      <p class="intro">所有被授权访问这个服务的人看到同一个数字。容器重启后状态仍然保留。</p>
      <div id="value" class="value" aria-live="polite">—</div>
      <div class="actions">
        <button data-action="decrement" aria-label="减一">−</button>
        <button data-action="increment" aria-label="加一">＋</button>
      </div>
      <button class="reset" data-action="reset">归零</button>
      <p id="status" class="status"></p>
    </section>
  </main>
  <script src="/demo/counter/assets/app.js" defer></script>
</body>
</html>"#;

const APP_CSS: &str = r#":root{color-scheme:dark;font-family:Inter,ui-sans-serif,system-ui,sans-serif;background:#120d18;color:#fff7ed}*{box-sizing:border-box}body{margin:0;min-height:100vh;background:radial-gradient(circle at 75% 10%,#64283e 0,transparent 42%),#120d18}main{width:min(680px,calc(100% - 32px));margin:0 auto;padding:48px 0}.back{color:#fdba74;text-decoration:none}.card{margin-top:24px;padding:clamp(24px,6vw,56px);text-align:center;border:1px solid #713f4d;border-radius:28px;background:#22131dcc;box-shadow:0 24px 80px #0008}.eyebrow{margin:0;color:#fda4af;font-size:.78rem;font-weight:800;letter-spacing:.14em}h1{margin:.5rem 0;font-size:clamp(2rem,7vw,4rem)}.intro{margin-inline:auto;max-width:500px;color:#d6bdc7;line-height:1.7}.value{margin:36px 0 24px;font-size:clamp(5rem,24vw,10rem);font-variant-numeric:tabular-nums;font-weight:900;line-height:1}.actions{display:flex;justify-content:center;gap:16px}button{border:1px solid #fb7185;border-radius:999px;background:#4c1d2c;color:#fff7ed;font:inherit;font-weight:900;cursor:pointer}.actions button{width:72px;height:58px;font-size:2rem}.reset{margin-top:18px;padding:10px 22px;border-color:#713f4d;background:transparent;color:#d6bdc7}.status{min-height:1.5em;margin:16px 0 0;color:#fda4af}"#;

const APP_JS: &str = r#"const value=document.querySelector('#value');const status=document.querySelector('#status');let busy=false;async function load(){const response=await fetch('/demo/counter/api/state');if(!response.ok)throw new Error('读取失败');const body=await response.json();value.textContent=body.value}async function mutate(action){if(busy)return;busy=true;status.textContent='正在保存…';try{const response=await fetch('/demo/counter/api/'+action,{method:'POST'});const body=await response.json();if(!response.ok)throw new Error(body.error||'保存失败');value.textContent=body.value;status.textContent='已持久化到本服务的 SQLite'}catch(error){status.textContent='操作失败：'+error.message}finally{busy=false}}document.querySelectorAll('[data-action]').forEach(button=>button.addEventListener('click',()=>mutate(button.dataset.action)));load().catch(error=>status.textContent=error.message);"#;

#[cfg(test)]
mod tests {
    use super::*;

    fn memory_database() -> Connection {
        let connection = Connection::open_in_memory().unwrap();
        connection
            .execute_batch(
                "CREATE TABLE counter (
                    id INTEGER PRIMARY KEY,
                    value INTEGER NOT NULL,
                    updated_at INTEGER NOT NULL
                 );
                 INSERT INTO counter VALUES (1, 0, unixepoch());",
            )
            .unwrap();
        connection
    }

    #[test]
    fn delta_is_persisted_and_clamped() {
        let mut database = memory_database();
        assert_eq!(apply_delta(&mut database, 1).unwrap().value, 1);
        assert_eq!(apply_delta(&mut database, -2).unwrap().value, -1);
        assert_eq!(
            apply_delta(&mut database, i64::MAX).unwrap().value,
            MAX_VALUE
        );
    }

    #[test]
    fn origin_must_match_exactly() {
        let state = AppState {
            database: Arc::new(Mutex::new(memory_database())),
            allowed_origins: parse_origins("https://111.229.205.99"),
        };
        let mut headers = HeaderMap::new();
        headers.insert(header::ORIGIN, "https://evil.example".parse().unwrap());
        assert!(matches!(
            require_origin(&state, &headers),
            Err(AppError::Forbidden)
        ));
        headers.insert(header::ORIGIN, "https://111.229.205.99".parse().unwrap());
        assert!(require_origin(&state, &headers).is_ok());
    }
}
