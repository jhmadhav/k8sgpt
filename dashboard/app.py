"""
Azure Extensions Health Copilot
A Streamlit chat UI for k8sgpt Azure Extensions analyzer.
"""

import os
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
        "- 📦 Core k8sgpt analyzers"
    )


def fetch_analysis(url, explain=False):
    """Call k8sgpt HTTP API (POST /v1/analyze)."""
    try:
        resp = requests.post(f"{url}/v1/analyze", json={"explain": explain}, timeout=60)
        resp.raise_for_status()
        return {"success": True, "data": resp.json()}
    except requests.exceptions.ConnectionError:
        return {"success": False, "msg": f"Cannot connect to k8sgpt at {url}"}
    except requests.exceptions.Timeout:
        return {"success": False, "msg": "Request timed out (60s)"}
    except Exception as e:
        return {"success": False, "msg": str(e)}


def is_azure_result(result):
    kind = result.get("kind", "")
    return "AzureExtension" in kind or "AzureArc" in kind


def severity_icon(result):
    texts = " ".join(e.get("text", "") for e in result.get("error", []))
    if "Failed" in texts or "CrashLoop" in texts or "OOMKilled" in texts:
        return "🔴"
    if "Pending" in texts or "Updating" in texts or "pending-install" in texts:
        return "🟡"
    if "NOT synced" in texts or "missing" in texts:
        return "🟠"
    return "⚠️"


def render_result(result):
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
st.caption(f"Connected to: `{api_url}` | ⏱ {datetime.now().strftime('%H:%M:%S')}")

col1, col2, col3 = st.columns(3)
with col1:
    scan_clicked = st.button("🔍 Scan Extensions", use_container_width=True, type="primary")
with col2:
    explain_clicked = st.button("🤖 Scan + AI Explain", use_container_width=True)
with col3:
    clear_clicked = st.button("🗑️ Clear", use_container_width=True)

if clear_clicked:
    st.session_state.pop("scan_result", None)
    st.rerun()

# Run scan
if scan_clicked or explain_clicked:
    with st.spinner("🔍 Analyzing cluster extensions..."):
        result = fetch_analysis(api_url, explain=explain_clicked)
    st.session_state["scan_result"] = result

# Display
if "scan_result" in st.session_state:
    result = st.session_state["scan_result"]

    if not result["success"]:
        st.error(f"❌ {result['msg']}")
    else:
        data = result["data"]
        results = data.get("results") or []
        api_errors = data.get("errors") or []
        problems = data.get("problems", 0)

        azure_results = [r for r in results if is_azure_result(r)]
        core_results = [r for r in results if not is_azure_result(r)]

        # Metrics
        m1, m2, m3, m4 = st.columns(4)
        m1.metric("🔷 Azure Issues", len(azure_results) if azure_results else "0 ✅")
        m2.metric("📊 Total Issues", problems)
        m3.metric("⬜ Core Issues", len(core_results))
        m4.metric("🔐 RBAC Warnings", len(api_errors))

        # Azure results
        st.markdown("## 🔷 Azure Extension Analysis")
        if azure_results:
            for r in azure_results:
                render_result(r)
        else:
            st.success("✅ All Azure extensions are healthy! No problems detected.")

        # RBAC warnings
        if api_errors:
            with st.expander(f"⚠️ {len(api_errors)} RBAC permission warnings", expanded=False):
                for e in api_errors:
                    st.caption(f"• {e}")

        # Core results
        if show_all and core_results:
            st.markdown("## ⬜ Core k8sgpt Analysis")
            for r in core_results:
                render_result(r)
else:
    st.info("👆 Click **Scan Extensions** to analyze your cluster.")
