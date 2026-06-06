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
    show_core = st.checkbox("Show core k8sgpt analysis", value=False)
    st.markdown("---")
    st.markdown("🔧 `AzureExtensionConfig`\n\n🛡️ `AzureArcAgents`")


def fetch_analysis(url, explain=False):
    try:
        resp = requests.post(f"{url}/v1/analyze", json={"explain": explain}, timeout=90)
        if resp.status_code == 500:
            return {"success": False, "msg": "AI backend error. Use 'Scan' without AI, or check AI provider config."}
        resp.raise_for_status()
        return {"success": True, "data": resp.json()}
    except requests.exceptions.ConnectionError:
        return {"success": False, "msg": f"Cannot connect to k8sgpt at {url}"}
    except Exception as e:
        return {"success": False, "msg": str(e)}


def is_azure(r):
    return "Azure" in r.get("kind", "")


def icon_for(r):
    t = " ".join(e.get("text", "") for e in r.get("error", []))
    if "Failed" in t or "CrashLoop" in t:
        return "🔴"
    if "Pending" in t or "Updating" in t:
        return "🟡"
    return "🟠"


# --- Main ---
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
    with st.spinner("🔍 Analyzing cluster extensions..."):
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
        problems = data.get("problems", 0)

        # Metrics
        m1, m2, m3, m4 = st.columns(4)
        m1.metric("🔷 Azure Issues", len(az_results) if az_results else "0 ✅")
        m2.metric("📊 Total Issues", problems)
        m3.metric("⬜ Core Issues", len(core_results))
        m4.metric("🔐 RBAC Warnings", len(api_errors))

        # Azure results
        st.markdown("## 🔷 Azure Extension Analysis")
        if az_results:
            for r in az_results:
                kind, name = r.get("kind", "?"), r.get("name", "?")
                with st.expander(f"{icon_for(r)} **{kind}** — `{name}`", expanded=True):
                    for e in r.get("error", []):
                        txt = e.get("text", "")
                        if "[HelmRelease]" in txt or "Helm release" in txt:
                            st.warning(f"⎈ {txt}")
                        elif "CrashLoop" in txt or "ImagePull" in txt:
                            st.error(f"💥 {txt}")
                        elif "[Deployment]" in txt:
                            st.warning(f"📦 {txt}")
                        elif "[K8sEvents]" in txt or "[Job]" in txt:
                            st.info(f"📋 {txt}")
                        elif "log errors" in txt.lower():
                            st.code(txt, language="text")
                        elif "missing" in txt.lower() or "synced" in txt.lower():
                            st.error(f"🔑 {txt}")
                        else:
                            st.warning(f"⚠️ {txt}")
                    if r.get("details"):
                        st.markdown("---")
                        st.success(f"🤖 **AI Explanation:**\n\n{r['details']}")
        else:
            st.success("✅ All Azure extensions are healthy! No problems detected.")

        if api_errors:
            with st.expander(f"⚠️ {len(api_errors)} RBAC warnings", expanded=False):
                for e in api_errors:
                    st.caption(f"• {e}")

        if show_core and core_results:
            st.markdown("## ⬜ Core Analysis")
            for r in core_results:
                with st.expander(f"⚠️ **{r.get('kind','?')}** — `{r.get('name','?')}`", expanded=False):
                    for e in r.get("error", []):
                        st.caption(e.get("text", ""))
                    if r.get("details"):
                        st.success(f"🤖 {r['details']}")
else:
    st.info("👆 Click **Scan Extensions** to analyze your cluster.")
