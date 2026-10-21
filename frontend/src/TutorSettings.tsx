import { FormEvent, useCallback, useEffect, useState } from "react";
import MCPSettings, { MCPReferencePicker } from "./MCPSettings";
import { api, Material, TutorAgent, TutorSkill } from "./api";

const example = "名称：数学解题引导\n适用条件：方程，去括号，移项，通分\n资料：数学\n工具：knowledge_search\n启用：是\n\n先判断学生已写步骤是否正确，指出错误所在，提示下一步检查方向。不要写最终得数或做完整题。依据资料时标注编号。";
const empty = { name: "", prompt: "", enabled: true, skill_ids: [] as number[], material_ids: [] as number[] };
export default function TutorSettings({ onError }: { onError: (error: unknown) => void }) {
  const [agents, setAgents] = useState<TutorAgent[]>([]), [skills, setSkills] = useState<TutorSkill[]>([]), [materials, setMaterials] = useState<Material[]>([]);
  const [agent, setAgent] = useState(empty), [agentID, setAgentID] = useState<number | undefined>(), [source, setSource] = useState(example), [skillID, setSkillID] = useState<number | undefined>();
  const [busy, setBusy] = useState(false), [loading, setLoading] = useState(true), [notice, setNotice] = useState(""), [error, setError] = useState("");
  const load = useCallback(async () => { const [a, s, m] = await Promise.all([api.agents(), api.skills(), api.materials("")]); setAgents(a.agents); setSkills(s.skills); setMaterials(m.materials); }, []);
  useEffect(() => { load().catch(onError).finally(() => setLoading(false)); }, [load, onError]);
  async function save(action: () => Promise<unknown>, message: string) { setBusy(true); setError(""); setNotice(""); try { await action(); await load(); setNotice(message); } catch (e) { setError(e instanceof Error ? e.message : "保存失败"); onError(e); } finally { setBusy(false); } }
  function toggle(field: "skill_ids" | "material_ids", id: number) { setAgent((old) => ({ ...old, [field]: old[field].includes(id) ? old[field].filter((n) => n !== id) : [...old[field], id] })); }
  function agentSubmit(e: FormEvent) { e.preventDefault(); void save(async () => { const result = await api.saveAgent(agent, agentID); setAgentID(result.id); setAgent(result); }, "助手配置已保存，下次提问生效。"); }
  function skillSubmit(e: FormEvent) { e.preventDefault(); void save(async () => { const result = await api.saveSkill(source, skillID); setSkillID(result.id); }, "技能已保存，下次提问生效。"); }
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
      <section className="settings-card"><p className="eyebrow">解题方式</p><h2>技能文件</h2><label>选择编辑的技能<select aria-label="编辑技能" value={skillID || 0} disabled={busy} onChange={(e) => { const item = skills.find((s) => s.id === Number(e.target.value)); setSkillID(item?.id); setSource(item?.source || example); }}><option value={0}>新建技能</option>{skills.map((s) => <option key={s.id} value={s.id}>{s.name}{!s.enabled && "（停用）"}</option>)}</select></label>
        <form onSubmit={skillSubmit}><label>导入 .txt / .md<input type="file" accept=".txt,.md" disabled={busy} onChange={async (e) => { const file = e.target.files?.[0]; if (!file) return; try { if (file.size > 32000) throw new Error("技能文件过大，最多 8000 字"); const bytes = await file.arrayBuffer(); setSource(new TextDecoder("utf-8", { fatal: true }).decode(bytes)); setSkillID(undefined); } catch (reason) { setError(reason instanceof Error ? reason.message : "文件读取失败"); } }} /></label>
          <label>技能原文<textarea className="skill-source" aria-label="技能原文" rows={14} value={source} onChange={(e) => setSource(e.target.value)} spellCheck={false} /></label><p className="muted">名称、适用条件、启用为必填；空行后写说明。把「启用：是」改为「启用：否」可停用。工具支持 knowledge_search 与 工具名@服务器名；保存即批准已核对的当前声明版本。</p>
          <MCPReferencePicker source={source} />
          <div className="settings-actions"><button className="primary" disabled={busy || loading}>保存技能</button>{skillID && <button type="button" className="secondary danger-action" disabled={busy} onClick={() => { if (confirm("删除此技能并清除助手绑定？")) void save(async () => { await api.deleteSkill(skillID); setSkillID(undefined); setSource(example); }, "技能已删除。"); }}>删除技能</button>}</div>
        </form></section></div>
    <MCPSettings onError={onError} />
    <a className="docs-link" href="/swagger/" target="_blank" rel="noreferrer">打开接口文档与试调 ↗</a>
  </section>;
}
