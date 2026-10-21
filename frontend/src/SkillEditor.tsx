import { FormEvent, useState } from "react";
import { api, SkillPreview, SkillRouting, TutorSkill } from "./api";
import { MCPReferencePicker } from "./MCPSettings";

const example = "名称：数学解题引导\n适用条件：方程，去括号，移项，通分\n资料：数学\n工具：knowledge_search\n启用：是\n\n先判断学生步骤是否正确，指出错误并提示下一步，不写最终得数。";
const defaults: SkillRouting = {trigger: "", subject: "", tools: [], enabled: true};
export default function SkillEditor({skills, onSaved, onError}: {skills: TutorSkill[]; onSaved: () => Promise<void>; onError: (error: unknown) => void}) {
 const [source,setSource]=useState(example),[id,setID]=useState<number>(),[routing,setRouting]=useState<SkillRouting>(defaults);
 const [preview,setPreview]=useState<SkillPreview>(),[busy,setBusy]=useState(false),[error,setError]=useState(""),[notice,setNotice]=useState("");
 const standard=source.replace(/\r\n/g,"\n").startsWith("---\n");
 async function action(work:()=>Promise<void>) {setBusy(true);setError("");setNotice("");try {await work();}catch(e){setError(e instanceof Error?e.message:"操作失败");onError(e);}finally{setBusy(false);}}
 async function inspect(raw=source) {setPreview(await api.previewSkill(raw));}
 function submit(e:FormEvent){e.preventDefault();void action(async()=>{const item=await api.saveSkill(source,id,standard?routing:undefined);setID(item.id);await onSaved();setNotice("技能已保存，下次提问生效。");});}
 return <section className="settings-card"><p className="eyebrow">解题方式</p><h2>技能文件</h2>
 {error&&<p className="form-error" role="alert">{error}</p>}{notice&&<p className="settings-notice" role="status">{notice}</p>}
 <label>选择编辑的技能<select aria-label="编辑技能" value={id||0} disabled={busy} onChange={e=>{const item=skills.find(s=>s.id===Number(e.target.value));setID(item?.id);setSource(item?.source||example);setRouting(item?{trigger:item.trigger,subject:item.subject,tools:item.tools,enabled:item.enabled}:defaults);setPreview(undefined);setNotice("");}}><option value={0}>新建技能</option>{skills.map(s=><option key={s.id} value={s.id}>{s.name}{!s.enabled&&"（停用）"}</option>)}</select></label>
 <form onSubmit={submit}><label>导入 .txt / .md<input type="file" accept=".txt,.md" disabled={busy} onChange={e=>{const file=e.target.files?.[0];if(!file)return;void action(async()=>{if(!/\.(txt|md)$/i.test(file.name))throw new Error("仅支持 .txt / .md 单文件");if(file.size>32000)throw new Error("技能文件过大，最多 8000 字");const raw=new TextDecoder("utf-8",{fatal:true}).decode(await file.arrayBuffer());setPreview(undefined);setSource(raw);setID(undefined);setRouting(defaults);await inspect(raw);});}}/></label>
 <p className="muted">支持课程中文字段和带 YAML frontmatter 的 SKILL.md，仅导入当前文件；不加载附属文件或执行脚本。</p>
 <label>技能原文<textarea className="skill-source" aria-label="技能原文" rows={14} value={source} onChange={e=>{setSource(e.target.value);setPreview(undefined);}} spellCheck={false}/></label>
 <button type="button" className="secondary" disabled={busy} onClick={()=>void action(()=>inspect())}>预览技能</button>
 {preview&&<section className="skill-preview" aria-label="技能解析预览"><h3>{preview.name}</h3><small>{preview.format==="skill-md"?"SKILL.md 标准格式":"课程格式"}</small><pre>{preview.description}</pre></section>}
 {standard?<fieldset><legend>标准技能运行配置</legend><label>触发关键词<input required maxLength={120} aria-label="技能触发关键词" value={routing.trigger} onChange={e=>setRouting({...routing,trigger:e.target.value})} placeholder="例如：方程，移项"/></label><label>资料标题范围<input maxLength={32} aria-label="技能资料范围" value={routing.subject} onChange={e=>setRouting({...routing,subject:e.target.value})}/></label><label>工具引用<input aria-label="技能工具引用" value={routing.tools.join("，")} onChange={e=>setRouting({...routing,tools:e.target.value.split(/[,，、;；|]/).map(x=>x.trim())})} placeholder="knowledge_search，formula_hint@campus-tools"/></label><label className="check-row"><input type="checkbox" checked={routing.enabled} onChange={e=>setRouting({...routing,enabled:e.target.checked})}/>启用标准技能</label><p className="muted">触发关键词用于匹配问题；文件中的工具元数据不授予权限。保存前请预览并核对工具声明。</p></fieldset>:<p className="muted">名称、适用条件、启用为必填；空行后写说明。启停请修改「启用：是/否」。工具支持 knowledge_search 与 工具名@服务器名。</p>}
 <MCPReferencePicker source={source} references={standard?routing.tools:undefined}/>
 <div className="settings-actions"><button className="primary" disabled={busy||(standard&&!preview)}>保存技能</button>{id&&<button type="button" className="secondary danger-action" disabled={busy} onClick={()=>{if(confirm("删除此技能并清除助手绑定？"))void action(async()=>{await api.deleteSkill(id);setID(undefined);setSource(example);setPreview(undefined);await onSaved();setNotice("技能已删除。");});}}>删除技能</button>}</div>
 </form></section>;
}
