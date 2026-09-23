'use strict';
(() => {
  const {node} = TaskLab;
  TaskLab.ready.then(session => {
    if (!['student','business'].includes(session.role)) return;
    const storageKey = `tasklab-chat-v1:${session.role}:${session.company || session.team_id || 'new'}`;
    let history = [], busy = false;
    try {
      const saved = JSON.parse(sessionStorage.getItem(storageKey) || '[]');
      if (Array.isArray(saved)) history = saved.filter(m => m && ['user','assistant'].includes(m.role) && typeof m.content === 'string' && m.content.length <= 8000).slice(-20);
    } catch { /* The chat also works without browser storage. */ }
    const launcher = node('button', undefined, 'chat-launcher'); launcher.type='button';
    const glyph = node('span','◌','chat-glyph'); glyph.setAttribute('aria-hidden','true');
    launcher.append(glyph,node('span','Помощник')); launcher.setAttribute('aria-label','Открыть чат с помощником');
    launcher.setAttribute('aria-controls','tasklab-chat'); launcher.setAttribute('aria-expanded','false');
    const panel = node('section', undefined, 'chat-panel'); panel.id='tasklab-chat'; panel.hidden=true;
    panel.setAttribute('role','dialog'); panel.setAttribute('aria-labelledby','chat-title');
    const head = node('div', undefined, 'chat-header');
    const heading = node('h2','Помощник TaskLab'); heading.id='chat-title';
    const close = node('button','×','chat-close');close.type='button';close.setAttribute('aria-label','Закрыть чат');
    head.append(heading,close);
    const messages = node('div',undefined,'chat-messages');messages.setAttribute('role','log');messages.setAttribute('aria-live','polite');messages.setAttribute('aria-label','Сообщения чата');
    const status = node('p','','chat-status');status.setAttribute('role','status');
    const error = node('p','','chat-error');error.setAttribute('role','alert');
    const form = node('form',undefined,'chat-form');
    const label = node('label','Ваш вопрос');label.htmlFor='chat-input';
    const input = node('textarea');input.id='chat-input';input.rows=3;input.maxLength=2000;input.required=true;input.placeholder='Напишите сообщение…';
    const actions = node('div',undefined,'chat-actions');
    const clear = node('button','Очистить чат','secondary');clear.type='button';
    const send = node('button','Отправить','primary');send.type='submit';actions.append(clear,send);
    form.append(label,input,actions);
    panel.append(head,messages,status,error,form);document.body.append(panel,launcher);
    function addMessage(role,text) {
      const bubble=node('div',undefined,`chat-message chat-${role}`);
      bubble.append(node('span',role==='user'?'Вы':'Помощник','chat-author'),node('p',text));messages.append(bubble);
      messages.scrollTop=messages.scrollHeight;return bubble;
    }
    function render() {
      messages.replaceChildren();
      const welcome=session.role==='business'?'Здесь можно задать вопрос о задачах, откликах и работе с командами.':'Здесь можно задать вопрос о задачах, своей команде и подготовке предложения.';
      messages.append(node('p',welcome,'chat-welcome'));
      for(const m of history) addMessage(m.role,m.content);
    }
    function save() {try {sessionStorage.setItem(storageKey,JSON.stringify(history));} catch {status.textContent='История доступна только до обновления страницы.';}}
    function toggle(open) {
      panel.hidden=!open;launcher.setAttribute('aria-expanded',String(open));
      launcher.setAttribute('aria-label',open?'Свернуть чат':'Открыть чат с помощником');
      if(open){messages.scrollTop=messages.scrollHeight;input.focus();}else launcher.focus();
    }
    launcher.addEventListener('click',()=>toggle(panel.hidden));close.addEventListener('click',()=>toggle(false));
    panel.addEventListener('keydown',event=>{if(event.key==='Escape'){event.preventDefault();toggle(false);}});
    input.addEventListener('keydown',event=>{if(event.key==='Enter'&&!event.shiftKey&&!event.isComposing){event.preventDefault();if(!busy)form.requestSubmit();}});
    clear.addEventListener('click',()=>{if(busy)return;history=[];save();render();error.textContent='';input.focus();});
    form.addEventListener('submit',async event=>{
      event.preventDefault();const message=input.value.trim();if(busy||!message)return;
      busy=true;send.disabled=true;clear.disabled=true;input.readOnly=true;error.textContent='';status.textContent='Помощник готовит ответ…';
      const pending=addMessage('user',message);
      const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),25000);
      const id=new URLSearchParams(location.search).get('id');
      const taskID=/^\/static\/task-builder\/(task|index)\.html$/.test(location.pathname)||location.pathname==='/static/task-builder/' ? (/^[1-9]\d*$/.test(id||'')&&Number.isSafeInteger(Number(id))?Number(id):null) : null;
      try {
        const response=await fetch('/api/chat',{method:'POST',credentials:'same-origin',signal:controller.signal,headers:{'Content-Type':'application/json'},body:JSON.stringify({message,history:history.slice(-12),context:{page:location.pathname,task_id:taskID}})});
        if([404,405,501,503].includes(response.status))throw new Error('Чатбот пока не подключён или временно недоступен. Ваш вопрос сохранён в поле — попробуйте позже.');
        let data;try{data=await response.json();}catch{throw new Error('Не удалось прочитать ответ помощника. Попробуйте ещё раз.');}
        if(!response.ok)throw new Error(typeof data.error==='string'?data.error:'Не удалось отправить сообщение.');
        if(typeof data.reply!=='string'||!data.reply.trim()||data.reply.length>8000)throw new Error('Помощник вернул некорректный ответ. Попробуйте ещё раз.');
        history.push({role:'user',content:message},{role:'assistant',content:data.reply});history=history.slice(-20);
        addMessage('assistant',data.reply);input.value='';status.textContent='';save();
      }catch(e){pending.remove();status.textContent='';error.textContent=e.name==='AbortError'?'Ответ не получен за 25 секунд. Попробуйте ещё раз.':e instanceof TypeError?'Нет связи с помощником. Проверьте соединение.':e.message;}
      finally {clearTimeout(timer);busy=false;send.disabled=false;clear.disabled=false;input.readOnly=false;if(!panel.hidden&&panel.contains(document.activeElement))input.focus();}
    });
    render();
  }).catch(() => { /* The shared navigation already displays the session error. */ });
})();
