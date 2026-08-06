#[tauri::command]
fn app_identity() -> &'static str {
    "abuzar-next"
}

const CREDENTIAL_SERVICE: &str = "com.abuzar.next";

#[tauri::command]
fn set_api_session(value: String) -> Result<(), String> {
    if value.is_empty() || value.len() > 4096 {
        return Err("session value is empty or too large".to_string());
    }
    let entry = keyring::Entry::new(CREDENTIAL_SERVICE, "api-session")
        .map_err(|error| format!("open Windows credential: {error}"))?;
    entry
        .set_password(&value)
        .map_err(|error| format!("store Windows credential: {error}"))
}

#[tauri::command]
fn get_api_session() -> Result<Option<String>, String> {
    let entry = keyring::Entry::new(CREDENTIAL_SERVICE, "api-session")
        .map_err(|error| format!("open Windows credential: {error}"))?;
    match entry.get_password() {
        Ok(value) => Ok(Some(value)),
        Err(keyring::Error::NoEntry) => Ok(None),
        Err(error) => Err(format!("read Windows credential: {error}")),
    }
}

#[tauri::command]
fn clear_api_session() -> Result<(), String> {
    let entry = keyring::Entry::new(CREDENTIAL_SERVICE, "api-session")
        .map_err(|error| format!("open Windows credential: {error}"))?;
    match entry.delete_credential() {
        Ok(()) | Err(keyring::Error::NoEntry) => Ok(()),
        Err(error) => Err(format!("clear Windows credential: {error}")),
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![app_identity, set_api_session, get_api_session, clear_api_session])
        .run(tauri::generate_context!())
        .expect("error while running Abuzar Next desktop application");
}
