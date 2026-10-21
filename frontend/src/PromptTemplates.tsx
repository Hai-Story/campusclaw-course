import { FormEvent, useEffect, useState } from "react";
import { api, PromptTemplate } from "./api";

export default function PromptTemplates({onApply,onError}:{onApply:(prompt:string)=>void;onError:(error:unknown)=>void}){
 const [items,setItems]=useState<PromptTemplate[]>([]),[id,setID]=useState(0),[name,setName]=useState(""),[prompt,setPrompt]=useState("");
 const [busy,setBusy]=useState(false),[error,setError]=useState(""),[notice,setNotice]=useState("");
 const selected=items.find(x=>x.id===id),readonly=!!selected?.builtin;
 async function load(){setItems((await api.promptTemplates()).templates);}
 useEffect(()=>{void load().catch(onError);},[onError]);
 async function action(work:()=>Promise<void>){setBusy(true);setError("");setNotice("");try{await work();await load();}catch(e){setError(e instanceof Error?e.message:"模板操作失败");onError(e);}finally{setBusy(false);}}
 function choose(value:number){const item=items.find(x=>x.id===value);setID(value);setName(item?.name||"");setPrompt(item?.prompt||"");setNotice("");}
 function save(e:FormEvent){e.preventDefault();void action(async()=>{const result=await api.savePromptTemplate({name,prompt},id||undefined);setID(result.id);setNotice("模板已保存；已配置的助手不会自动改变。");});}
 return <section className="prompt-templates" aria-label="提示词模板库"><h3>提示词模板库</h3>
 {error&&<p role="alert" className="form-error">{error}</p>}{notice&&<p role="status" className="settings-notice">{notice}</p>}
 <label>选择提示词模板<select aria-label="选择提示词模板" value={id} disabled={busy} onChange={e=>choose(Number(e.target.value))}><option value={0}>新建自定义模板</option>{items.map(x=><option key={x.id} value={x.id}>{x.name}{x.builtin?"（内置）":""}</option>)}</select></label>
 <form onSubmit={save}><label>模板名称<input aria-label="模板名称" required maxLength={64} value={name} readOnly={readonly} onChange={e=>setName(e.target.value)}/></label><label>模板预览与正文<textarea aria-label="模板正文" rows={4} required maxLength={2000} readOnly={readonly} value={prompt} onChange={e=>setPrompt(e.target.value)}/></label>
 <div className="settings-actions"><button type="button" className="secondary" disabled={busy||!prompt.trim()} onClick={()=>onApply(prompt)}>应用到角色提示词</button><button type="button" className="secondary" disabled={busy||!id} onClick={()=>{setID(0);setName(name.slice(0,60)+"副本");setNotice("已复制，请填写名称并保存。");}}>复制为自定义</button>{!readonly&&<button className="primary" disabled={busy}>保存模板</button>}{id>0&&!readonly&&<button type="button" className="secondary danger-action" disabled={busy} onClick={()=>{if(confirm("删除此模板？已有助手提示词将保留。"))void action(async()=>{await api.deletePromptTemplate(id);choose(0);setNotice("模板已删除。");});}}>删除模板</button>}</div></form>
 <p className="muted">内置模板只读。应用仅复制正文，保存助手后生效；修改或删除模板不会改变已有助手。</p></section>;
}
