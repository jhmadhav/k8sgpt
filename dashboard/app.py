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
    st.image("https://img.icons8.com/fluency/96/microsoft-azure-2019.png", width=64)
    st.title("⚙️ Settings")
    api_url = st.text_input("k8sgpt API URL", value=K8SGPT_API)
    auto_refresh = st.checkbox("Auto-refresh (30s)", value=False)
    st.markdown("---")
    st.markdown("### About")
    st.markdown(
        "**Azure Extensions Health Copilot** uses [k8sgpt](https://k8sgpt.ai) "
        "with a custom Azure Extensions analyzer to diagnose extension failures "
        "on AKS and Arc-enabled clusters."
    )
    st.markdown("---")
    st.markdown(
        "**Analyzers:**\n"
        "- 🔧 `AzureExtensionConfig` — Extension CRDs\n"
        "- 🛡️ `AzureArcAgents` — Arc agent health"
    )


def fetch_analysis(url: str, explain: bool = False) -> dict:
    """Call k8sgpt HTTP API to get analysis results."""
    try:
        params = {"explain": "true"} if explain else {}
        # k8sgpt serve --http exposes REST at /v1/analyze
        resp = requests.get(f"{url}/v1/analyze", params=params, timeout=30)
        resp.raise_for_status()
        return resp.json()
    except requests.exceptions.ConnectionError:
        return {"error": f"Cannot connect to k8sgpt at {url}"}
    except requests.exceptions.Timeout:
        return {"error": "Request timed out"}
    except Exception as e:
        return {"error": str(e)}


def render_result(result: dict):
    """Render a single analysis result as a card."""
    kind = result.get("kind", "Unknown")
    name = result.get("name", "Unknown")
    errors = result.get("error", [])
    details = result.get("details", "")
    parent = result.get("parentObject", "")

    # Determine severity icon
    if any("Failed" in e.get("text", "") for e in errors):
        icon = "🔴"
    elif any("Pending" in e.get("text", "") or "Updating" in e.get("text", "") for e in errors):
        icon = "🟡"
    else:
        icon = "🟠"

    with st.expander(f"{icon} **{kind}** — `{name}`", expanded=True):
        if parent:
            st.caption(f"Parent: {parent}")

        for err in errors:
            text = err.get("text", "")
            if "ExtensionEvent" in text:
                st.info(f"📋 {text}")
            elif "Pod" in text and ("log" in text.lower() or "CrashLoop" in text):
                st.error(f"🪵 {text}")
            elif "Helm" in text:
                st.warning(f"⎈ {text}")
            else:
                st.warning(f"⚠️ {text}")

        if details:
            st.markdown("### 🤖 AI Explanation")
            st.markdown(details)


# --- Main UI ---
st.title("🔍 Azure Extensions Health Copilot")
st.caption(f"Connected to: `{api_url}` | Last check: {datetime.now().strftime('%H:%M:%S')}")

# Chat-style interaction
if "messages" not in st.session_state:
    st.session_state.messages = []

# Display chat history
for msg in st.session_state.messages:
    with st.chat_message(msg["role"]):
        st.markdown(msg["content"])

# Chat input
user_input = st.chat_input("Ask about your extensions... (e.g., 'scan my cluster', 'what's failing?')")

if user_input or auto_refresh:
    query = user_input or "Auto-refresh scan"

    # Show user message
    if user_input:
        st.session_state.messages.append({"role": "user", "content": query})
        with st.chat_message("user"):
            st.markdown(query)

    # Fetch results
    with st.chat_message("assistant"):
        with st.spinner("🔍 Analyzing extensions..."):
            data = fetch_analysis(api_url, explain="explain" in query.lower())

        if "error" in data:
            response = f"❌ **Error:** {data['error']}"
            st.error(response)
        else:
            results = data.get("results", [])
            if not results:
                response = "✅ **All extensions are healthy!** No problems detected."
                st.success(response)
            else:
                response = f"Found **{len(results)} issue(s)** across your extensions:"
                st.markdown(response)
                for r in results:
                    render_result(r)

        st.session_state.messages.append({"role": "assistant", "content": response})

# Quick action buttons
st.markdown("---")
col1, col2, col3 = st.columns(3)
with col1:
    if st.button("🔍 Scan Now", use_container_width=True):
        st.session_state.messages.append({"role": "user", "content": "Scan my cluster"})
        st.rerun()
with col2:
    if st.button("🤖 Scan + Explain (AI)", use_container_width=True):
        st.session_state.messages.append({"role": "user", "content": "Scan and explain my cluster"})
        st.rerun()
with col3:
    if st.button("🗑️ Clear Chat", use_container_width=True):
        st.session_state.messages = []
        st.rerun()

if auto_refresh:
    import time
    time.sleep(30)
    st.rerun()
