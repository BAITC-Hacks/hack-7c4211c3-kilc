'use strict';
(() => {
  const { node } = window.TaskLab;
  const icons = {
    catalog: 'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z',
    team: 'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2 M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8 M22 21v-2a4 4 0 0 0-3-3.87 M16 3.13a4 4 0 0 1 0 7.75',
    plus: 'M12 5v14 M5 12h14',
    business: 'M3 7h18v14H3z M8 7V3h8v4 M3 12h18 M10 12v3h4v-3'
  };
  function icon(name) {
    const svg = document.createElementNS('http://www.w3.org/2000/svg','svg');
    for (const [key,value] of Object.entries({viewBox:'0 0 24 24',fill:'none',stroke:'currentColor','stroke-width':'1.7','stroke-linecap':'round','stroke-linejoin':'round','aria-hidden':'true'})) svg.setAttribute(key,value);
    const path = document.createElementNS(svg.namespaceURI,'path'); path.setAttribute('d', icons[name]); svg.append(path); return svg;
  }
  function render(session) {
    const nav = document.querySelector('.nav'); if (!nav) return;
    nav.replaceChildren(); document.body.dataset.role = session.role;
    const items = session.role === 'student' ? [ ['/', 'Каталог задач','catalog'], ['/static/task-builder/teams.html','Моя команда','team'] ] : session.role === 'business' ? [ ['/static/task-builder/business.html','Мои задачи','business'], ['/static/task-builder/team-catalog.html', 'Каталог команд','team'], ['/static/task-builder/index.html','Создать задачу','plus'] ] : [];
    let current = location.pathname;
    if (current.endsWith('/task.html')) current = '/';
    if (current === '/static/task-builder/') current += 'index.html';
    for (const [href,label,glyph] of items) {
      const link = node('a'); link.href = href; link.append(icon(glyph),node('span',label));
      if (href === current) link.setAttribute('aria-current','page'); nav.append(link);
    }
    const badge = document.querySelector('#session-badge');
    badge.textContent = session.role === 'business' ? (session.company || 'Бизнес') : session.role === 'student' ? 'Студент' : 'Добро пожаловать';
    document.querySelector('.switch-role')?.remove();
    if (session.role) {
      const leave = node('button', 'Сменить роль', 'secondary switch-role'); leave.type = 'button';
      leave.addEventListener('click', async () => {
        if (!confirm('Сменить роль? Задачи, команды и отклики сохранятся; в этом браузере нужно будет снова выбрать роль.')) return;
        leave.disabled = true;
        try {
          const response = await fetch('/api/session', { method: 'DELETE', credentials: 'same-origin' });
          if (!response.ok) throw new Error('Не удалось сменить роль. Попробуйте ещё раз.');
          try { sessionStorage.clear(); } catch { /* Role selection works without browser storage. */ }
          location.href = '/static/task-builder/login.html';
        } catch (error) { leave.disabled = false; alert(error.message); }
      });
      badge.after(leave);
    }
  }
  TaskLab.ready.then(render).catch(error => {
    const nav = document.querySelector('.nav'); if (!nav) return;
    const message = node('span',error.message,'message'); message.setAttribute('role','alert');
    const retry = node('button','Повторить','secondary'); retry.addEventListener('click',()=>location.reload()); nav.replaceChildren(message,retry);
  });
  document.addEventListener('team:selected',()=>render(TaskLab.session));
  // Native cross-document view transitions leave link semantics and browser history intact.
  document.addEventListener('pointerdown', event => {
    const target = event.target.closest('button,.nav a,.clickable-card,.card');
    if (!target || target.disabled || matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    target.animate([{transform:'scale(1)'},{transform:'scale(.98)'},{transform:'scale(1)'}], {duration:200,easing:'ease-out'});
  });
})();
