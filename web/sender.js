const $ = selector => document.querySelector(selector);
const key = 'abi-sender-session';
let session = null, families = [], drafts = [], busy = false, activeSend = null, deleting = null;
try { session = JSON.parse(sessionStorage.getItem(key) || localStorage.getItem(key) || 'null'); } catch {}
const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const myId = () => session?.id || '';
const unresolved = f => ['sending','uncertain','whatsapp_opened'].includes(f.status);
const notice = text => { $('#notice').textContent = text; };
function forget() {
  session = null; families = []; drafts = []; activeSend = null; deleting = null;
  try { sessionStorage.removeItem(key); localStorage.removeItem(key); } catch {}
  $('#roster').replaceChildren(); $('#drafts').replaceChildren();
  document.querySelectorAll('dialog[open]').forEach(d => d.close());
  $('#signin').hidden = false; $('#workspace').hidden = $('#review').hidden = $('#signout').hidden = true;
}
async function api(path, method = 'GET', body) {
  const response = await fetch('/api' + path, {
    method, credentials:'omit', headers: { ...(session ? {Authorization:'Bearer '+session.token} : {}), ...(body ? {'Content-Type':'application/json'} : {}) },
    body:body ? JSON.stringify(body) : undefined,
  });
  const data = await response.json().catch(()=>({}));
  if (!response.ok) {
    if (response.status === 401 && session) forget();
    throw Error(response.status === 401 ? 'Sign in with your username and a new PIN from the administrator.' : response.status === 404 ? 'This item or feature is unavailable. Refresh, or ask the administrator to deploy the latest backend.' : ({400:'Check the entered details and try again.',403:'You do not have permission for this action.',409:'The roster changed or this action is unavailable. Refresh and review the assignment and sending status.'}[response.status] || 'The server could not complete this request. Try again.') + ` (HTTP ${response.status})`);
  }
  return data;
}
async function task(work) {
  if (busy) return;
  busy = true;
  const controls = [...document.querySelectorAll('button,input,select')].filter(e=>!e.disabled);
  controls.forEach(e=>e.disabled=true);
  try { await work(); } catch(error) { notice(error.message); if ($('#send-dialog').open) $('#send-notice').textContent=error.message; }
  finally { busy=false; controls.forEach(e=>{if(e.isConnected)e.disabled=false;}); }
}
function normalize(phone, country='') {
  let number=phone.trim().replace(/[ ().-]/g,'');
  if (!/^(\+|00)?[0-9]+$/.test(number)) return null;
  if(number.startsWith('00')) number='+'+number.slice(2);
  else if(!number.startsWith('+')) { if(number.length!==10 || !/^\+[1-9][0-9]{0,2}$/.test(country))return null; number=country+number; }
  if(number.startsWith('+521')&&number.length===14)number='+52'+number.slice(4);
  return /^\+[1-9][0-9]{7,14}$/.test(number)?number:null;
}
async function refresh() {
  families=(await api('/families')).families;
  $('#signin').hidden=true; $('#signout').hidden=false;
  if($('#review').hidden)$('#workspace').hidden=false;
  $('#signed-in-as').textContent='SIGNED IN AS '+session.username;
  renderRoster();
}
function statusLabel(value) { return ({pending:'Not sent',sending:'SMS prepared — verify sending',whatsapp_opened:'WhatsApp prepared — verify sending',uncertain:'Needs verification',failed:'Not sent / failed',manual_sent:'Sent (user confirmed)',sent:'SMS sent',delivered:'SMS delivered'})[value]||value; }
function renderRoster() {
  const mine=families.filter(f=>f.assignedTo===myId());
  $('#summary').textContent=`${families.length} families · ${mine.length} assigned to you · ${mine.filter(unresolved).length} to verify`;
  const query=$('#search').value.toLowerCase(), filter=$('#filter').value;
  const shown=families.filter(f=>(f.name+' '+f.phone).toLowerCase().includes(query)&&
    (filter==='all'||filter==='mine'&&f.assignedTo===myId()||filter==='unassigned'&&!f.assignedTo||filter==='review'&&f.assignedTo===myId()&&unresolved(f)));
  $('#roster').innerHTML=shown.map(f=>{
    const owner=f.assignedTo===myId();
    const response=f.rsvp==='yes'?(f.headcountUnknown?'Attending · headcount pending':'Attending'):f.rsvp==='no'?'Not attending':'No RSVP yet';
    return `<article class="family-card" data-id="${esc(f.id)}"><h2>${esc(f.name)}</h2><p>${esc(f.phone)}</p><p>${esc(response)} · ${esc(statusLabel(f.status))}</p><p class="muted">Assigned to: ${esc(f.assignedName||'Nobody')}</p><div class="sender-actions">${!f.assignedTo?'<button class="button gold" data-action="claim">Assign to me</button>':''}${owner?`${['pending','failed'].includes(f.status)?'<button class="button gold" data-action="sms">Prepare SMS</button><button class="button outline" data-action="whatsapp">Prepare WhatsApp</button>':''}${unresolved(f)?'<button class="button gold" data-action="verify">Review sending</button>':''}${!unresolved(f)?'<button class="button outline" data-action="release">Release assignment</button><button class="button danger" data-action="delete">Delete family</button>':''}`:''}</div></article>`;
  }).join('')||'<p>No families match this view.</p>';
}
$('#signin-form').onsubmit=event=>{
  event.preventDefault(); task(async()=>{
    const username=$('#username').value.trim().toLowerCase(), pin=$('#pin').value;
    const result=await api('/user/login','POST',{username,pin});
    session={username:result.username,id:result.id,token:result.token}; $('#pin').value='';
    try { sessionStorage.removeItem(key);localStorage.removeItem(key);($('#remember').checked?localStorage:sessionStorage).setItem(key,JSON.stringify(session)); }
    catch {notice('Signed in for this page only; browser storage is unavailable.');}
    await refresh();notice('Signed in. Choose contacts or claim an existing family.');
  });
};
$('#signout').onclick=()=>{if(confirm('Sign out? You will need a new single-use PIN to sign in again.'))task(async()=>{await api('/user/logout','POST',{});forget();notice('Signed out. Ask the administrator for a new PIN when needed.');});};
$('#refresh').onclick=()=>task(refresh);
$('#search').oninput=renderRoster;$('#filter').onchange=renderRoster;
const action=(f,body)=>api(`/families/${f.id}/actions`,'POST',body);
function invitationMessage(f) { return `¡Hola, ${f.name}! Celebremos los dos años de Abi el 24 de octubre a las 4:00 p. m. ¡Trae a tu familia! Confirma aquí: ${location.origin}/?i=${f.id}`; }
function showSend(f) {
  activeSend=f; $('#send-family').textContent=f.name+' · '+f.phone;
  const message=invitationMessage(f);$('#message-preview').value=message;
  const link=$('#open-message');
  link.href=f.channel==='whatsapp'?`https://wa.me/${f.phone.replace(/\D/g,'')}?text=${encodeURIComponent(message)}`:`sms:${f.phone}${/iPhone|iPad|iPod/.test(navigator.userAgent)?'&':'?'}body=${encodeURIComponent(message)}`;
  link.textContent=f.channel==='whatsapp'?'Open WhatsApp ↗':'Open SMS app ↗';
  link.target=f.channel==='whatsapp'?'_blank':'_self';link.rel='noopener noreferrer';
  $('#send-notice').textContent='';$('#send-dialog').showModal();
}
$('#roster').onclick=event=>{
  const button=event.target.closest('button[data-action]');if(!button||busy)return;
  const f=families.find(f=>f.id===button.closest('[data-id]').dataset.id);if(!f)return;
  const command=button.dataset.action;
  if(command==='delete') {deleting=f;$('#delete-description').textContent=`Remove ${f.name} from the shared roster? This deletes RSVP and sending history and invalidates the invitation link. Your phone contact is not deleted.`;$('#delete-dialog').showModal();return;}
  if(command==='verify'){showSend(f);return;}
  task(async()=>{
    if(command==='claim'||command==='release'){await action(f,{action:command});await refresh();return;}
    const prepared=await action(f,{action:'begin',channel:command,attemptId:crypto.randomUUID()});
    await refresh();showSend(prepared);
  });
};
$('#copy-message').onclick=()=>task(async()=>{await navigator.clipboard.writeText($('#message-preview').value);$('#send-notice').textContent='Message copied. Sending is still unconfirmed.';});
$('#send-close').onclick=()=>$('#send-dialog').close();
function finishSend(sent) {
  if(!activeSend||!confirm(sent?'Confirm you actually pressed Send in the messaging app?':'Confirm this message was not sent? This will allow a new attempt.'))return;
  task(async()=>{await action(activeSend,{action:'result',attemptId:activeSend.attemptId,status:sent?'manual_sent':'failed',detail:sent?'User confirmed sending from web':'User confirmed message was not sent from web'});$('#send-dialog').close();activeSend=null;await refresh();notice(sent?'Marked sent by you.':'Marked not sent. You can prepare a new message.');});
}
$('#mark-sent').onclick=()=>finishSend(true);$('#mark-canceled').onclick=()=>finishSend(false);
$('#delete-cancel').onclick=()=>$('#delete-dialog').close();
$('#delete-confirm').onclick=()=>task(async()=>{if(!deleting)return;await api(`/families/${deleting.id}`,'DELETE');$('#delete-dialog').close();deleting=null;await refresh();notice('Family deleted from roster.');});
for(const dialog of document.querySelectorAll('dialog'))dialog.addEventListener('cancel',event=>{if(busy)event.preventDefault();});
function newDraft(contact={}) {
  const phones=[...new Set((contact.tel||[]).filter(Boolean))];
  return {id:crypto.randomUUID(),name:contact.name?.[0]||'',phone:phones.length===1?phones[0]:'',phones,country:'',email:contact.email?.[0]||'',status:'',message:''};
}
function showReview() {$('#workspace').hidden=true;$('#review').hidden=false;renderDrafts();}
function renderDrafts() {
  $('#drafts').innerHTML=drafts.map(d=>{
    const locked=['added','existing'].includes(d.status), disabled=locked?'disabled':'';
    return `<article class="draft-card" data-id="${d.id}"><label>Head of family<input data-field="name" value="${esc(d.name)}" ${disabled} /></label>${d.phones.length>1&&!locked?`<label>Choose a phone number<select data-field="choice"><option value="">Choose one</option>${d.phones.map(p=>`<option value="${esc(p)}" ${p===d.phone?'selected':''}>${esc(p)}</option>`).join('')}</select></label>`:''}<label>Phone<input data-field="phone" type="tel" value="${esc(d.phone)}" ${disabled} /></label><label>Country code for local numbers<input data-field="country" type="tel" placeholder="Blank until selected; + numbers keep their code" value="${esc(d.country)}" ${disabled} /></label>${!locked?'<div class="country-shortcuts"><button class="button outline" data-country="+1">US +1</button><button class="button outline" data-country="+52">Mexico +52</button></div>':''}<label>Email (optional)<input data-field="email" type="email" value="${esc(d.email)}" ${disabled} /></label><p class="draft-number"></p><p>${esc(d.message)}</p>${!locked?'<button class="button outline" data-remove>Remove</button>':''}</article>`;
  }).join('');
  updateNumbers();
}
function updateNumbers() {
  for(const card of $('#drafts').children){const d=drafts.find(d=>d.id===card.dataset.id);card.querySelector('.draft-number').textContent=normalize(d.phone,d.country)||'Choose a country code or enter a full international number.';}
}
$('#drafts').oninput=event=>{
  const field=event.target.dataset.field;if(!field||field==='choice')return;
  const d=drafts.find(d=>d.id===event.target.closest('[data-id]').dataset.id);d[field]=event.target.value;updateNumbers();
};
$('#drafts').onchange=event=>{if(event.target.dataset.field==='choice'){const d=drafts.find(d=>d.id===event.target.closest('[data-id]').dataset.id);d.phone=event.target.value;d.country='';renderDrafts();}};
$('#drafts').onclick=event=>{
  const button=event.target.closest('button');if(!button||busy)return;
  const id=button.closest('[data-id]').dataset.id;
  if(button.hasAttribute('data-remove'))drafts=drafts.filter(d=>d.id!==id);
  else if(button.dataset.country)drafts.find(d=>d.id===id).country=button.dataset.country;
  renderDrafts();
};
const contactSupport=Boolean(navigator.contacts?.select&&window.ContactsManager);
$('#pick').hidden=!contactSupport;
$('#contacts-help').textContent=contactSupport?'Your browser lets you choose multiple contacts. Only contacts you review and import are uploaded.':'Phone contact selection is unavailable in this browser. Add families manually, or use Chrome on Android for contact selection.';
$('#pick').onclick=async()=>{
  if(busy)return;
  // Keep this call directly in the user gesture; browser permission is per selection.
  try {
    const supported=await navigator.contacts.getProperties();
    const contacts=await navigator.contacts.select(['name','tel','email'].filter(p=>supported.includes(p)),{multiple:true});
    if(!contacts.length)return;
    if(contacts.length>100){notice('Choose at most 100 contacts at a time. Nothing was imported.');return;}
    drafts=contacts.map(newDraft);showReview();notice('Review each family before adding it. No messages will be sent.');
  }catch(error){notice(error.name==='AbortError'?'Contact selection canceled.':'Could not open contacts. You can add families manually.');}
};
$('#manual').onclick=()=>{drafts=[newDraft()];showReview();};
$('#add-another').onclick=()=>{if(drafts.length>=100){notice('Maximum 100 contacts per import.');return;}drafts.push(newDraft());renderDrafts();};
$('#close-review').onclick=()=>{drafts=[];$('#drafts').replaceChildren();$('#review').hidden=true;$('#workspace').hidden=false;task(refresh);};
$('#import').onclick=()=>task(async()=>{
  const pending=drafts.filter(d=>!['added','existing'].includes(d.status));
  if(!pending.length){notice('Nothing left to import. Return to the roster.');return;}
  for(const d of pending){if(!d.name.trim()||new TextEncoder().encode(d.name.trim()).length>120||!normalize(d.phone,d.country)){throw Error('Every family needs a name (up to 120 UTF-8 bytes) and a valid international phone number.');}}
  for(let start=0;start<pending.length;start+=20){
    const chunk=pending.slice(start,start+20);
    const response=await api('/families/import','POST',{contacts:chunk.map(d=>({name:d.name.trim(),phone:normalize(d.phone,d.country),email:d.email.trim()}))});
    for(const result of response.results){const d=chunk[result.index];if(!d)continue;d.status=result.status;d.message=result.status==='added'?'Added and assigned to you':result.status==='existing'?'Already in roster — unchanged':result.message||'Not confirmed; retry safely';}
  }
  notice(`${drafts.filter(d=>d.status==='added').length} added · ${drafts.filter(d=>d.status==='existing').length} existing numbers skipped. Any unconfirmed contacts can be retried safely.`);
}).finally(()=>renderDrafts());
if(session?.token){task(refresh);}else{forget();}
