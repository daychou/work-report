use std::{
    fs,
    os::unix::fs::PermissionsExt,
    path::{Path, PathBuf},
    process::Command,
};

use chacha20poly1305::{
    aead::{rand_core::RngCore, Aead, AeadCore, KeyInit, OsRng},
    ChaCha20Poly1305, Key, Nonce,
};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use tauri::{
    menu::{Menu, MenuItem},
    tray::TrayIconBuilder,
    AppHandle, Manager, RunEvent, WindowEvent,
};
use tauri_plugin_autostart::ManagerExt;

const CREDENTIAL_FILE: &str = "credentials.bin";
const SALT_FILE: &str = "credentials.salt";
const KEY_CONTEXT: &[u8] = b"work-report-desktop-api-key-v1";
const NONCE_LENGTH: usize = 12;

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(default)]
struct DesktopPreferences {
    server_url: String,
    recent_project_id: Option<u64>,
    launch_at_startup: bool,
}

fn config_dir(app: &AppHandle) -> Result<PathBuf, String> {
    app.path()
        .app_config_dir()
        .map_err(|error| format!("无法定位配置目录：{error}"))
}

fn preferences_path(app: &AppHandle) -> Result<PathBuf, String> {
    Ok(config_dir(app)?.join("preferences.json"))
}

fn read_preferences(app: &AppHandle) -> DesktopPreferences {
    preferences_path(app)
        .ok()
        .and_then(|path| fs::read_to_string(path).ok())
        .and_then(|content| serde_json::from_str(&content).ok())
        .unwrap_or_default()
}

fn write_preferences(app: &AppHandle, preferences: &DesktopPreferences) -> Result<(), String> {
    let path = preferences_path(app)?;
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|error| format!("无法创建配置目录：{error}"))?;
    }
    let content = serde_json::to_vec_pretty(preferences)
        .map_err(|error| format!("无法序列化配置：{error}"))?;
    let temporary_path = path.with_extension("json.tmp");
    fs::write(&temporary_path, content).map_err(|error| format!("无法写入配置：{error}"))?;
    fs::rename(temporary_path, path).map_err(|error| format!("无法保存配置：{error}"))
}

/// 机器指纹参与密钥派生，凭据文件被复制到另一台机器后无法解密。
fn machine_fingerprint() -> String {
    let uuid = Command::new("/usr/sbin/ioreg")
        .args(["-rd1", "-c", "IOPlatformExpertDevice"])
        .output()
        .ok()
        .and_then(|output| {
            String::from_utf8_lossy(&output.stdout)
                .lines()
                .find_map(|line| line.split_once("\"IOPlatformUUID\" = "))
                .map(|(_, value)| value.trim().trim_matches('"').to_string())
        });
    uuid.unwrap_or_else(|| "unknown-machine".into())
}

/// 目录 0700、文件 0600：凭据的实际保护来自文件权限和全盘加密。
fn private_dir(app: &AppHandle) -> Result<PathBuf, String> {
    let dir = config_dir(app)?;
    fs::create_dir_all(&dir).map_err(|error| format!("无法创建配置目录：{error}"))?;
    fs::set_permissions(&dir, fs::Permissions::from_mode(0o700))
        .map_err(|error| format!("无法收紧配置目录权限：{error}"))?;
    Ok(dir)
}

fn write_private(path: &Path, bytes: &[u8]) -> Result<(), String> {
    let temporary_path = path.with_extension("tmp");
    fs::write(&temporary_path, bytes).map_err(|error| format!("无法写入凭据：{error}"))?;
    fs::set_permissions(&temporary_path, fs::Permissions::from_mode(0o600))
        .map_err(|error| format!("无法收紧凭据权限：{error}"))?;
    fs::rename(temporary_path, path).map_err(|error| format!("无法保存凭据：{error}"))
}

fn credential_salt(app: &AppHandle) -> Result<Vec<u8>, String> {
    let path = private_dir(app)?.join(SALT_FILE);
    if let Ok(salt) = fs::read(&path) {
        if salt.len() == 32 {
            return Ok(salt);
        }
    }
    let mut salt = [0u8; 32];
    OsRng.fill_bytes(&mut salt);
    write_private(&path, &salt)?;
    Ok(salt.to_vec())
}

fn credential_cipher(app: &AppHandle) -> Result<ChaCha20Poly1305, String> {
    let mut hasher = Sha256::new();
    hasher.update(KEY_CONTEXT);
    hasher.update(machine_fingerprint().as_bytes());
    hasher.update(credential_salt(app)?);
    let key = hasher.finalize();
    Ok(ChaCha20Poly1305::new(Key::from_slice(&key)))
}

fn seal(cipher: &ChaCha20Poly1305, plaintext: &str) -> Result<Vec<u8>, String> {
    let nonce = ChaCha20Poly1305::generate_nonce(&mut OsRng);
    let ciphertext = cipher
        .encrypt(&nonce, plaintext.as_bytes())
        .map_err(|_| "无法加密凭据".to_string())?;
    let mut payload = nonce.to_vec();
    payload.extend_from_slice(&ciphertext);
    Ok(payload)
}

fn open(cipher: &ChaCha20Poly1305, payload: &[u8]) -> Result<String, String> {
    if payload.len() <= NONCE_LENGTH {
        return Err("凭据文件已损坏，请重新绑定".into());
    }
    let (nonce, ciphertext) = payload.split_at(NONCE_LENGTH);
    let plaintext = cipher
        .decrypt(Nonce::from_slice(nonce), ciphertext)
        .map_err(|_| "凭据无法解密，请重新绑定".to_string())?;
    String::from_utf8(plaintext).map_err(|_| "凭据内容无效，请重新绑定".to_string())
}

fn read_api_key(app: &AppHandle) -> Result<String, String> {
    let path = config_dir(app)?.join(CREDENTIAL_FILE);
    let payload = fs::read(&path).map_err(|_| "本机还没有绑定 API Key".to_string())?;
    open(&credential_cipher(app)?, &payload)
}

#[tauri::command]
fn get_preferences(app: AppHandle) -> DesktopPreferences {
    read_preferences(&app)
}

#[tauri::command]
fn set_preferences(app: AppHandle, preferences: DesktopPreferences) -> Result<(), String> {
    write_preferences(&app, &preferences)
}

#[tauri::command]
fn save_api_key(app: AppHandle, api_key: String) -> Result<(), String> {
    if !api_key.starts_with("wrk_") {
        return Err("API Key 格式无效".into());
    }
    let payload = seal(&credential_cipher(&app)?, &api_key)?;
    write_private(&private_dir(&app)?.join(CREDENTIAL_FILE), &payload)
}

#[tauri::command]
fn get_api_key(app: AppHandle) -> Result<String, String> {
    read_api_key(&app)
}

#[tauri::command]
fn has_api_key(app: AppHandle) -> bool {
    read_api_key(&app)
        .map(|key| key.starts_with("wrk_"))
        .unwrap_or(false)
}

#[tauri::command]
fn delete_api_key(app: AppHandle) -> Result<(), String> {
    let path = config_dir(&app)?.join(CREDENTIAL_FILE);
    match fs::remove_file(&path) {
        Ok(()) => Ok(()),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Err(error) => Err(format!("无法移除本机凭据：{error}")),
    }
}

fn show_window(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.show();
        let _ = window.unminimize();
        let _ = window.set_focus();
    }
}

#[tauri::command]
fn set_launch_at_startup(app: AppHandle, enabled: bool) -> Result<(), String> {
    let autolaunch = app.autolaunch();
    if enabled {
        autolaunch.enable()
    } else {
        autolaunch.disable()
    }
    .map_err(|error| format!("无法修改开机启动设置：{error}"))
}

fn build_tray(app: &tauri::App) -> tauri::Result<()> {
    let show = MenuItem::with_id(app, "show", "显示主界面", true, None::<&str>)?;
    let quit = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&show, &quit])?;

    let mut builder = TrayIconBuilder::new()
        .menu(&menu)
        .show_menu_on_left_click(true)
        .tooltip("Workline 快捷工作台")
        .on_menu_event(|app, event| match event.id.as_ref() {
            "show" => show_window(app),
            "quit" => app.exit(0),
            _ => {}
        });

    if let Some(icon) = app.default_window_icon() {
        builder = builder.icon(icon.clone());
    }
    builder.build(app)?;
    Ok(())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _, _| {
            show_window(app);
        }))
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_autostart::Builder::new().build())
        .setup(|app| {
            build_tray(app)?;
            Ok(())
        })
        .on_window_event(|window, event| {
            // 关闭按钮只收起前台界面，客户端继续在托盘中驻留。
            if let WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .invoke_handler(tauri::generate_handler![
            get_preferences,
            set_preferences,
            save_api_key,
            get_api_key,
            has_api_key,
            delete_api_key,
            set_launch_at_startup,
        ])
        .build(tauri::generate_context!())
        .expect("Workline 客户端启动失败")
        .run(|app, event| {
            // macOS 上再次点击 Dock / Finder 图标不会启动新进程，只会触发 Reopen。
            if let RunEvent::Reopen { .. } = event {
                show_window(app);
            }
        });
}

#[cfg(test)]
mod tests {
    use super::{open, seal, DesktopPreferences};
    use chacha20poly1305::{aead::KeyInit, ChaCha20Poly1305, Key};

    fn test_cipher(seed: u8) -> ChaCha20Poly1305 {
        ChaCha20Poly1305::new(Key::from_slice(&[seed; 32]))
    }

    #[test]
    fn sealed_api_key_round_trips() {
        let cipher = test_cipher(7);
        let payload = seal(&cipher, "wrk_example_key").unwrap();

        assert!(!payload
            .windows(15)
            .any(|window| window == b"wrk_example_key"));
        assert_eq!(open(&cipher, &payload).unwrap(), "wrk_example_key");
    }

    #[test]
    fn sealed_api_key_rejects_other_machine_and_tampering() {
        let payload = seal(&test_cipher(7), "wrk_example_key").unwrap();

        assert!(open(&test_cipher(8), &payload).is_err());
        assert!(open(&test_cipher(7), &payload[..payload.len() - 1]).is_err());
        assert!(open(&test_cipher(7), b"short").is_err());
    }

    #[test]
    fn preferences_ignore_removed_shortcut_fields_from_old_config() {
        let preferences: DesktopPreferences = serde_json::from_str(
            r#"{"server_url":"https://work.example.com","show_shortcut":"Alt+CommandOrControl+W"}"#,
        )
        .unwrap();

        assert_eq!(preferences.server_url, "https://work.example.com");
        assert_eq!(preferences.recent_project_id, None);
        assert!(!preferences.launch_at_startup);
    }
}
