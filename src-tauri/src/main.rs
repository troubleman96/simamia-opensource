use std::{
    io::{self, BufRead, BufReader, Write},
    net::{SocketAddr, TcpStream},
    path::PathBuf,
    sync::Mutex,
    thread,
    time::{Duration, Instant},
};

use tauri::{Manager, RunEvent};
use tauri_plugin_shell::{
    process::{CommandChild, CommandEvent},
    ShellExt,
};

struct ApiProcess(Mutex<Option<CommandChild>>);

fn api_is_ready() -> bool {
    let address = SocketAddr::from(([127, 0, 0, 1], 8080));
    let Ok(mut stream) = TcpStream::connect_timeout(&address, Duration::from_millis(250)) else {
        return false;
    };
    let _ = stream.set_read_timeout(Some(Duration::from_millis(500)));
    if stream
        .write_all(b"GET /api/v1/health HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n")
        .is_err()
    {
        return false;
    }

    let mut response = String::new();
    BufReader::new(stream).read_line(&mut response).is_ok()
        && (response.starts_with("HTTP/1.1 200") || response.starts_with("HTTP/1.0 200"))
}

fn start_api(app: &mut tauri::App) -> Result<(), Box<dyn std::error::Error>> {
    let app_data_dir = app.path().app_data_dir()?;
    std::fs::create_dir_all(&app_data_dir)?;
    let database_path: PathBuf = app_data_dir.join("simamia.db");
    let app_handle = app.handle().clone();

    let command = app_handle
        .shell()
        .sidecar("simamia-api")?
        .env("PORT", "8080")
        .env("SIMAMIA_LISTEN_ADDRESS", "127.0.0.1")
        .env(
            "SIMAMIA_DB_PATH",
            database_path.to_string_lossy().into_owned(),
        )
        .env(
            "CORS_ALLOWED_ORIGINS",
            "http://tauri.localhost,tauri://localhost,http://localhost:3000",
        )
        .env("LOG_LEVEL", "info");

    let (mut events, child) = command.spawn()?;
    app_handle.manage(ApiProcess(Mutex::new(Some(child))));

    tauri::async_runtime::spawn(async move {
        while let Some(event) = events.recv().await {
            match event {
                CommandEvent::Stdout(line) => {
                    println!("simamia-api: {}", String::from_utf8_lossy(&line).trim_end());
                }
                CommandEvent::Stderr(line) => {
                    eprintln!("simamia-api: {}", String::from_utf8_lossy(&line).trim_end());
                }
                CommandEvent::Error(error) => eprintln!("simamia-api sidecar error: {error}"),
                CommandEvent::Terminated(status) => {
                    eprintln!("simamia-api sidecar terminated: {status:?}");
                }
                _ => {}
            }
        }
    });

    let deadline = Instant::now() + Duration::from_secs(12);
    while Instant::now() < deadline {
        if api_is_ready() {
            return Ok(());
        }
        thread::sleep(Duration::from_millis(150));
    }

    Err(Box::new(io::Error::new(
        io::ErrorKind::TimedOut,
        "The bundled Simamia API did not become ready on 127.0.0.1:8080",
    )))
}

fn stop_api(app: &tauri::AppHandle) {
    if let Some(state) = app.try_state::<ApiProcess>() {
        if let Ok(mut process) = state.0.lock() {
            if let Some(child) = process.take() {
                let _ = child.kill();
            }
        }
    }
}

fn main() {
    let app = tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .setup(start_api)
        .build(tauri::generate_context!())
        .expect("error while running Simamia desktop");
    app.run(|app, event| {
        if let RunEvent::Exit = event {
            stop_api(app);
        }
    });
}
