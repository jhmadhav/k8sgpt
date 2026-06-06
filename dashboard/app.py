"""Azure Extensions Health Copilot — Streamlit UI for k8sgpt"""

import os
import requests
import streamlit as st
from datetime import datetime

K8SGPT_API = os.getenv("K8SGPT_API_URL", "http://k8sgpt-ext.k8sgpt-system.svc.cluster.local:8080")

st.set_page_config(page_title="Azure Extensions Health Copilot", page_icon="🔍", layout="wide")

with st.sidebar:
    st.title("⚙️ Settings")
    api_url = st.text_input("k8sgpt API URL", value=K8SGPT_API)
    show_all = st.checkbox("Show all analyzers", value=False)
    st.markdown("---")
    st.markdown("🔧 `AzureExtensionConfig`\n\n🛡️ `AzureArcAgents`")


def fetch_analysis(url, explain=False):
    try:
        body = {}
        if explain:
            body["explain"] = True
        resp = requests.post(f"{url}/v1/analyze", json=body, timeout=60)
        if resp.status_code == 500:
            return {"success": False, "msg": "AI backend not configured. Use 'Scan Extensions' without AI explain, or configure an AI provider."}
        resp.raise_for_status()
        return {"success": True, "data": resp.json()}
    except requests.exceptions.ConnectionError:
        return {"success": False, "msg": f"Cannot connect to k8sgpt at {url}"}
    except requests.exceptions.HTTPError as e:
        return {"success": False, "msg": f"HTTP error: {e}"}
    except Exception as e:
        return {"success": False, "msg": str(e)}


def is_azure(r):
    return "Azure" in r.get("kind", "")


def icon(r):
    t = " ".join(e.get("text", "") for e in r.get("error", []))
    if "Failed" in t or "CrashLoop" in t:
        return "🔴"
    if "Pending" in t or "Updating" in t or "pending" in t:
        return "🟡"
    if "synced" in t or "missing" in t:
        return "🟠"
    return "⚠️"


def render(r):
    k, n = r.get("kind", "?"), r.get("name", "?")
    az = is_azure(r)
    tag = "🔷 Azure" if az else "⬜ Core"
    with st.expander(f"{icon(r)} {tag} **{k}** — `{n}`", expanded=az):
        for e in r.get("error", []):
            txt = e.get("text", "")
            if "Helm" in txt:
                st.warning(f"⎈ {txt}")
            elif "CrashLoop" in txt or "ImagePull" in txt:
                st.error(f"💥 {txt}")
            elif "missing" in txt or "synced" in txt:
                st.error(f"🔑 {txt}")
            elif "log errors" in txt.lower():
                st.code(txt, language="text")
            else:
                st.warning(f"⚠️ {txt}")
        if r.get("details"):
            st.success(f"🤖 {r['details']}")


st.title("🔍 Azure Extensions Health Copilot")
st.caption(f"`{api_url}` | ⏱ {datetime.now().strftime('%H:%M:%S')}")

c1, c2, c3 = st.columns(3)
scan = c1.button("🔍 Scan Extensions", use_container_width=True, type="primary")
explain = c2.button("🤖 Scan + AI Explain", use_container_width=True)
clear = c3.button("🗑️ Clear", use_container_width=True)

if clear:
    st.session_state.pop("result", None)
    st.rerun()

if scan or explain:
    with st.spinner("🔍 Analyzing..."):
        st.session_state["result"] = fetch_analysis(api_url, explain=explain)

if "result" in st.session_state:
    res = st.session_state["result"]
    if not res["success"]:
        st.error(f"❌ {res['msg']}")
    else:
        data = res["data"]
        all_results = data.get("results") or []
        api_errors = data.get("errors") or []
        az_results = [r for r in all_results if is_azure(r)]
        core_results = [r for r in all_results if not is_azure(r)]

        m1, m2, m3, m4 = st.columns(4)
        m1.metric("🔷 Azure Issues", len(az_results) if az_results else "0 ✅")
        m2.metric("📊 Total", data.get("problems", 0))
        m3.metric("⬜ Core", len(core_results))
        m4.metric("🔐 RBAC", len(api_errors))

        st.markdown("## 🔷 Azure Extensions")
        if az_results:
            for r in az_results:
                render(r)
        else:
            st.success("✅ All Azure extensions are healthy!")

        if api_errors:
            with st.expander(f"⚠️ {len(api_errors)} RBAC warnings", expanded=False):
                for e in api_errors:
                    st.caption(f"• {e}")

        if show_all and core_results:
            st.markdown("## ⬜ Core Analysis")
            for r in core_results:
                render(r)
else:
    st.info("👆 Click **Scan Extensions** to analyze your cluster.")
