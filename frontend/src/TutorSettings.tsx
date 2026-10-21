import { FormEvent, useCallback, useEffect, useState } from "react";
import SkillEditor from "./SkillEditor";
import MCPSettings from "./MCPSettings";
import { api, Material, TutorAgent, TutorSkill } from "./api";

const empty = { name: "", prompt: "", enabled: true, skill_ids: [] as number[], material_ids: [] as number[] };
export default function TutorSettings({ onError }: { onError: (error: unknown) => void }) {
  const [agents, setAgents] = useState<TutorAgent[]>([]), [skills, setSkills] = useState<TutorSkill[]>([]), [materials, setMaterials] = useState<Material[]>([]);
  const [agent, setAgent] = useState(empty), [agentID, setAgentID] = useState<number | undefined>();
  const [busy, setBusy] = useState(false), [loading, setLoading] = useState(true), [notice, setNotice] = useState(""), [error, setError] = useState("");
  const load = useCallback(async () => { const [a, s, m] = await Promise.all([api.agents(), api.skills(), api.materials("")]); setAgents(a.agents); setSkills(s.skills); setMaterials(m.materials); }, []);
  useEffect(() => { load().catch(onError).finally(() => setLoading(false)); }, [load, onError]);
  async function save(action: () => Promise<unknown>, message: string) { setBusy(true); setError(""); setNotice(""); try { await action(); await load(); setNotice(message); } catch (e) { setError(e instanceof Error ? e.message : "保存失败"); onError(e); } finally { setBusy(false); } }
  function toggle(field: "skill_ids" | "material_ids", id: number) { setAgent((old) => ({ ...old, [field]: old[field].includes(id) ? old[field].filter((n) => n !== id) : [...old[field], id] })); }
  function agentSubmit(e: FormEvent) { e.preventDefault(); void save(async () => { const result = await api.saveAgent(agent, agentID); setAgentID(result.id); setAgent(result); }, "助手配置已保存，下次提问生效。"); }
  return <section className="tutor-settings">
    {loading && <p className="muted">正在读取本班配置…</p>}{error && <p className="form-error" role="alert">{error}</p>}{notice && <p className="settings-notice" role="status">{notice}</p>}
    <div className="settings-columns"><section className="settings-card"><p className="eyebrow">角色与资料</p><h2>智能体配置</h2>
      <label>选择编辑的助手<select aria-label="编辑助手" value={agentID || 0} disabled={busy} onChange={(e) => { const item = agents.find((a) => a.id === Number(e.target.value)); setAgentID(item?.id); setAgent(item || empty); setNotice(""); }}><option value={0}>新建助手</option>{agents.map((a) => <option value={a.id} key={a.id}>{a.name}{!a.enabled && "（停用）"}</option>)}</select></label>
      <form onSubmit={agentSubmit}><label>助手名称<input required aria-label="助手名称" value={agent.name} onChange={(e) => setAgent({ ...agent, name: e.target.value })} placeholder="例如：数学学习助手" /></label>
        <label>角色提示词<textarea aria-label="角色提示词" rows={5} value={agent.prompt} onChange={(e) => setAgent({ ...agent, prompt: e.target.value })} placeholder="留空使用默认中文解题助手；最多 2000 字" /></label>
        <label className="check-row"><input type="checkbox" checked={agent.enabled} onChange={(e) => setAgent({ ...agent, enabled: e.target.checked })} />启用助手（停用只阻止新建会话）</label>
        <fieldset><legend>绑定技能</legend>{skills.map((s) => <label className="check-row" key={s.id}><input type="checkbox" checked={agent.skill_ids.includes(s.id)} onChange={() => toggle("skill_ids", s.id)} />{s.name}{!s.enabled && "（停用）"}</label>)}{!skills.length && <p className="muted">先在右侧创建技能。</p>}</fieldset>
        <fieldset><legend>绑定本班材料</legend>{materials.map((m) => <label className="check-row" key={m.id}><input type="checkbox" checked={agent.material_ids.includes(m.id)} onChange={() => toggle("material_ids", m.id)} />{m.title}</label>)}<p className="muted">不绑定材料也可回答，回答将不带资料引用。</p></fieldset>
        <button className="primary" disabled={busy || loading}>保存助手</button>
      </form></section>
      <SkillEditor skills={skills} onSaved={load} onError={onError} /></div>
    <MCPSettings onError={onError} />
    <a className="docs-link" href="/swagger/" target="_blank" rel="noreferrer">打开接口文档与试调 ↗</a>
  </section>;
}
