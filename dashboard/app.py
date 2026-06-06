"""
Azure Extensions Health Copilot
A Streamlit chat UI for k8sgpt Azure Extensions analyzer.
"""

import os
import json
import requests
import streamlit as st
from datetime import datetime

K8SGPT_API = os.getenv("K8SGPT_API_URL", "http://k8sgpt-ext.k8sgpt-system.svc.cluster.local:8080")

st.set_page_config(
    page_title="Azure Extensions Health Copilot",
    page_icon="🔍",
    layout="wide",
)

# --- Sidebar ---
with st.sidebar:
    st.title("⚙️ Azure Extensions Health Copilot")
    api_url = st.text_input("k8sgpt API URL", value=K8SGPT_API)
    show_all = st.checkbox("Show all analyzers (not just Azure)", value=False)
    st.markdown("---")
    st.markdown(
        "**Analyzers:**\n"
        "- 🔧 `AzureExtensionConfig` — Extension CRs\n"
        "- 🛡️ `AzureArcAgents` — Arc agent health\n"
        "- 📦 Core k8sgpt analyzers (ConfigMap, Pod, etc.)"
    )


def fetch_analysis(url: str, explain: bool = False) -> dict:
    """Call k8sgpt HTTP API (POST /v1/analyze)."""
    try:
        body = {"explain": explain}
        resp = requests.post(f"{url}/v1/analyze", json=body, timeout=60)
        resp.raise_for_status()
        return resp.json()
    except requests.exceptions.ConnectionError:
        return {"error": f"Cannot connect to k8sgpt at {url}"}
    except requests.exceptions.Timeout:
        return {"error": "Request timed out (60s)"}
    except Exception as e:
        return {"error": str(e)}


def is_azure_result(result: dict) -> bool:
    kind = result.get("kind", "")
    return "AzureExtension" in kind or "AzureArc" in kind


def severity_icon(result: dict) -> str:
    errors = result.get("error", [])
    texts = " ".join(e.get("text", "") for e in errors)
    if "Failed" in texts or "CrashLoop" in texts or "OOMKilled" in texts:
        return "🔴"
    if "Pending" in texts or "Updating" in texts or "pending-install" in texts:
        return "🟡"
    if "NOT synced" in texts or "missing" in texts:
        return "🟠"
    return "⚠️"


def render_result(result: dict):
    kind = result.get("kind", "Unknown")
    name = result.get("name", "Unknown")
    errors = result.get("error", [])
    details = result.get("details", "")
    icon = severity_icon(result)

    is_azure = is_azure_result(result)
    badge = "🔷 Azure" if is_azure else "⬜ Core"

    with st.expander(f"{icon} {badge} **{kind}** — `{name}`", expanded=is_azure):
        for err in errors:
            text = err.get("text", "")
            if "ExtensionEvent" in text:
                st.info(f"📋 {text}")
            elif "log errors" in text.lower():
                st.code(text, language="text")
            elif "CrashLoop" in text or "ImagePull" in text or "OOMKilled" in text:
                st.error(f"💥 {text}")
            elif "Helm" in text:
                st.warning(f"⎈ {text}")
            elif "missing" in text.lower() or "not synced" in text.lower():
                st.error(f"🔑 {text}")
            else:
                st.warning(f"⚠️ {text}")

        if details:
            st.markdown("---")
            st.markdown("### 🤖 AI Explanation")
            st.success(details)


# --- Main ---
st.title("🔍 Azure Extensions Health Copilot")

col_status, col_time = st.columns([3, 1])
with col_time:
    st.caption(f"⏱ {datetime.now().strftime('%H:%M:%S')}")

# Action buttons
col1, col2, col3 = st.columns(3)
with col1:
    scan_clicked = st.button("🔍 Scan Extensions", use_container_width=True, type="primary")
with col2:
    explain_clicked = st.button("🤖 Scan + AI Explain", use_container_width=True)
with col3:
    clear_clicked = st.button("🗑️ Clear", use_container_width=True)

if clear_clicked:
    if "last_scan" in st.session_state:
        del st.session_state["last_scan"]
    st.rerun()

if scan_clicked or explain_clicked:
    with st.spinner("🔍 Analyzing cluster extensions..."):
        data = fetch_analysis(api_url, explain=explain_clicked)
    st.session_state["last_scan"] = data

# Display results
if "last_scan" in st.session_state:
    data = st.session_state["last_scan"]

    if "error" in data and isinstance(data["error"], str):
        st.error(f"❌ {data['error']}")
    else:
        results = data.get("results", [])
        api_errors = data.get("errors", [])
        status = data.get("status", "OK")
        problems = data.get("problems", 0)

        # Split results
        azure_results = [r for r in results if is_azure_result(r)]
        core_results = [r for r in results if not is_azure_result(r)]

        # Summary metrics
        m1, m2, m3, m4 = st.columns(4)
        with m1:
            if azure_results:
                st.metric("Azure Extension Issues", len(azure_results), delta=None)
            else:
                st.metric("Azure Extension Issues", "0 ✅")
        with m2:
            st.metric("Total Issues", problems)
        with m3:
            st.metric("Core Issues", len(core_results))
        with m4:
            st.metric("RBAC Errors", len(api_errors))

        # Azure extension results (always shown)
        st.markdown("## 🔷 Azure Extension Analysis")
        if azure_results:
            for r in azure_results:
                render_result(r)
        else:
            st.success("✅ All Azure extensions are healthy!")

        # RBAC warnings
        if api_errors:
            with st.expander(f"⚠️ {len(api_errors)} RBAC permission warnings", expanded=False):
                for e in api_errors:
                    st.caption(f"• {e}")

        # Core results (optional)
        if show_all and core_results:
            st.markdown("## ⬜ Core k8sgpt Analysis")
            for r in core_results:
                render_result(r)
else:
    st.info("👆 Click **Scan Extensions** to analyze your cluster.")
