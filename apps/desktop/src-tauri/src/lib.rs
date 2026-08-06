use reqwest::{Method, Url};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use std::time::Duration;

#[tauri::command]
fn app_identity() -> &'static str {
    "abuzar-next"
}

const CREDENTIAL_SERVICE: &str = "com.abuzar.next";
const EDGE_CONFIG_ACCOUNT: &str = "edge-config";

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StoredEdgeConfig {
    edge_url: String,
    shared_secret: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct EdgeConfigView {
    edge_url: Option<String>,
    shared_secret_configured: bool,
}

#[derive(Debug, Serialize)]
struct HardwareCommandError {
    code: String,
    status: Option<u16>,
    message: String,
}

impl HardwareCommandError {
    fn local(code: &str, message: impl Into<String>) -> Self {
        Self {
            code: code.to_string(),
            status: None,
            message: message.into(),
        }
    }
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct HardwareCapability {
    name: String,
    available: bool,
    provider: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    reason: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
struct HardwareCapabilitiesResponse {
    capabilities: Vec<HardwareCapability>,
}

#[derive(Debug, Default, Serialize, Deserialize)]
#[serde(default)]
#[serde(rename_all = "camelCase")]
struct SaleSlipLine {
    item_name: String,
    quantity: String,
    total: String,
}

#[derive(Debug, Default, Serialize, Deserialize)]
#[serde(default)]
#[serde(rename_all = "camelCase")]
struct SaleSlip {
    header: String,
    store: String,
    invoice_number: String,
    date: String,
    customer: String,
    lines: Vec<SaleSlipLine>,
    subtotal: String,
    discount: String,
    tax: String,
    total: String,
    footer: String,
}

#[derive(Debug, Default, Serialize, Deserialize)]
#[serde(default)]
#[serde(rename_all = "camelCase")]
struct PurchaseLabel {
    item_name: String,
    batch: String,
    expiry: String,
    mrp: String,
    quantity: String,
}

#[derive(Debug, Default, Serialize, Deserialize)]
#[serde(default)]
#[serde(rename_all = "camelCase")]
struct PurchaseLabelBatch {
    labels: Vec<PurchaseLabel>,
    cut_after: bool,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PrintResult {
    printed: bool,
    bytes: usize,
    provider: String,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct BarcodeItem {
    code: String,
    item_id: String,
    name: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct CashDrawerResult {
    kicked: bool,
}

#[derive(Debug, Deserialize)]
struct EdgeProblem {
    code: Option<String>,
    detail: Option<String>,
}

fn credential_entry() -> Result<keyring::Entry, HardwareCommandError> {
    keyring::Entry::new(CREDENTIAL_SERVICE, EDGE_CONFIG_ACCOUNT).map_err(|error| {
        HardwareCommandError::local(
            "credential_store_error",
            format!("Open edge configuration: {error}"),
        )
    })
}

fn read_edge_config() -> Result<Option<StoredEdgeConfig>, HardwareCommandError> {
    let entry = credential_entry()?;
    let value = match entry.get_password() {
        Ok(value) => value,
        Err(keyring::Error::NoEntry) => return Ok(None),
        Err(error) => {
            return Err(HardwareCommandError::local(
                "credential_store_error",
                format!("Read edge configuration: {error}"),
            ))
        }
    };
    serde_json::from_str(&value).map(Some).map_err(|_| {
        HardwareCommandError::local(
            "credential_store_error",
            "Stored edge configuration is invalid.",
        )
    })
}

fn validate_edge_url(value: &str) -> Result<Url, HardwareCommandError> {
    if value.trim() != value || value.is_empty() {
        return Err(HardwareCommandError::local(
            "invalid_edge_configuration",
            "The branch-edge URL must be a non-empty URL without surrounding whitespace.",
        ));
    }
    let url = Url::parse(value).map_err(|_| {
        HardwareCommandError::local(
            "invalid_edge_configuration",
            "The branch-edge URL must be a valid HTTP or HTTPS URL.",
        )
    })?;
    if !matches!(url.scheme(), "http" | "https")
        || url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
        || url.query().is_some()
        || url.fragment().is_some()
    {
        return Err(HardwareCommandError::local(
            "invalid_edge_configuration",
            "The branch-edge URL must use HTTP(S), include a host, and contain no credentials, query, or fragment.",
        ));
    }
    Ok(url)
}

fn configured_edge() -> Result<StoredEdgeConfig, HardwareCommandError> {
    read_edge_config()?.ok_or_else(|| {
        HardwareCommandError::local(
            "edge_not_configured",
            "Configure the branch-edge URL before using a hardware adapter.",
        )
    })
}

#[tauri::command]
fn set_edge_config(
    edge_url: String,
    shared_secret: String,
) -> Result<EdgeConfigView, HardwareCommandError> {
    validate_edge_url(&edge_url)?;
    if shared_secret.len() > 4096 {
        return Err(HardwareCommandError::local(
            "invalid_edge_configuration",
            "The branch-edge shared secret is too large.",
        ));
    }
    let config = StoredEdgeConfig {
        edge_url,
        shared_secret,
    };
    let value = serde_json::to_string(&config).map_err(|_| {
        HardwareCommandError::local("credential_store_error", "Encode edge configuration.")
    })?;
    credential_entry()?.set_password(&value).map_err(|error| {
        HardwareCommandError::local(
            "credential_store_error",
            format!("Store edge configuration: {error}"),
        )
    })?;
    Ok(EdgeConfigView {
        edge_url: Some(config.edge_url),
        shared_secret_configured: !config.shared_secret.is_empty(),
    })
}

#[tauri::command]
fn get_edge_config() -> Result<EdgeConfigView, HardwareCommandError> {
    match read_edge_config()? {
        Some(config) => Ok(EdgeConfigView {
            edge_url: Some(config.edge_url),
            shared_secret_configured: !config.shared_secret.is_empty(),
        }),
        None => Ok(EdgeConfigView {
            edge_url: None,
            shared_secret_configured: false,
        }),
    }
}

#[tauri::command]
fn clear_edge_config() -> Result<(), HardwareCommandError> {
    match credential_entry()?.delete_credential() {
        Ok(()) | Err(keyring::Error::NoEntry) => Ok(()),
        Err(error) => Err(HardwareCommandError::local(
            "credential_store_error",
            format!("Clear edge configuration: {error}"),
        )),
    }
}

async fn edge_request<B, T>(
    method: Method,
    path: &str,
    body: Option<&B>,
) -> Result<T, HardwareCommandError>
where
    B: Serialize + ?Sized,
    T: DeserializeOwned,
{
    let config = configured_edge()?;
    let base_url = validate_edge_url(&config.edge_url)?;
    let url = format!("{}{}", base_url.as_str().trim_end_matches('/'), path);
    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(10))
        .build()
        .map_err(|_| {
            HardwareCommandError::local(
                "edge_client_error",
                "Unable to initialize the branch-edge client.",
            )
        })?;
    let mut request = client
        .request(method, url)
        .header("accept", "application/json");
    if !config.shared_secret.is_empty() {
        request = request.bearer_auth(config.shared_secret);
    }
    if let Some(body) = body {
        request = request.json(body);
    }
    let response = request.send().await.map_err(|_| {
        HardwareCommandError::local(
            "edge_unreachable",
            "The configured branch-edge service could not be reached.",
        )
    })?;
    let status = response.status();
    if !status.is_success() {
        let problem = response.json::<EdgeProblem>().await.ok();
        return Err(HardwareCommandError {
            code: problem
                .as_ref()
                .and_then(|value| value.code.clone())
                .unwrap_or_else(|| "edge_request_failed".to_string()),
            status: Some(status.as_u16()),
            message: problem.and_then(|value| value.detail).unwrap_or_else(|| {
                format!(
                    "Branch-edge request failed with status {}.",
                    status.as_u16()
                )
            }),
        });
    }
    response.json::<T>().await.map_err(|_| {
        HardwareCommandError::local(
            "edge_invalid_response",
            "The branch-edge service returned an invalid hardware response.",
        )
    })
}

#[tauri::command]
async fn get_hardware_capabilities() -> Result<HardwareCapabilitiesResponse, HardwareCommandError> {
    edge_request::<(), HardwareCapabilitiesResponse>(Method::GET, "/v1/hardware/capabilities", None)
        .await
}

#[tauri::command]
async fn print_sale_slip(slip: SaleSlip) -> Result<PrintResult, HardwareCommandError> {
    edge_request(Method::POST, "/v1/hardware/print/sale-slip", Some(&slip)).await
}

#[tauri::command]
async fn print_purchase_labels(
    batch: PurchaseLabelBatch,
) -> Result<PrintResult, HardwareCommandError> {
    edge_request(
        Method::POST,
        "/v1/hardware/print/purchase-labels",
        Some(&batch),
    )
    .await
}

#[tauri::command]
async fn lookup_barcode(raw: String) -> Result<BarcodeItem, HardwareCommandError> {
    #[derive(Serialize)]
    struct BarcodeRequest {
        raw: String,
    }
    edge_request(
        Method::POST,
        "/v1/hardware/barcode/lookup",
        Some(&BarcodeRequest { raw }),
    )
    .await
}

#[tauri::command]
async fn kick_cash_drawer() -> Result<CashDrawerResult, HardwareCommandError> {
    edge_request::<(), CashDrawerResult>(Method::POST, "/v1/hardware/cash-drawer/kick", None).await
}

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
        .invoke_handler(tauri::generate_handler![
            app_identity,
            set_api_session,
            get_api_session,
            clear_api_session,
            set_edge_config,
            get_edge_config,
            clear_edge_config,
            get_hardware_capabilities,
            print_sale_slip,
            print_purchase_labels,
            lookup_barcode,
            kick_cash_drawer
        ])
        .run(tauri::generate_context!())
        .expect("error while running Abuzar Next desktop application");
}

#[cfg(test)]
mod tests {
    use super::validate_edge_url;

    #[test]
    fn accepts_explicit_http_edge_url() {
        assert!(validate_edge_url("http://127.0.0.1:8091").is_ok());
        assert!(validate_edge_url("https://edge.example.test").is_ok());
    }

    #[test]
    fn rejects_edge_url_credentials_and_ambiguous_suffixes() {
        for value in [
            "http://user:password@127.0.0.1:8091",
            "http://127.0.0.1:8091?secret=value",
            "http://127.0.0.1:8091#hardware",
            "ftp://127.0.0.1:8091",
            "not a url",
        ] {
            assert!(validate_edge_url(value).is_err(), "accepted {value}");
        }
    }
}
