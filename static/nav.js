'use strict';

(() => {
  const links = [...document.querySelectorAll('.nav a')];
  let pathname = window.location.pathname;
  if (pathname === '/static/task-builder/') pathname = '/static/task-builder/index.html';
  if (pathname === '/static/task-builder/task.html') pathname = '/';
  for (const link of links) {
    const linkPath = new URL(link.href, window.location.href).pathname;
    if (linkPath === pathname) link.setAttribute('aria-current', 'page');
    else link.removeAttribute('aria-current');
  }

  const roleLink = document.querySelector('[data-role-link]');
  if (!roleLink) return;
  let role = '';
  let teamID = '';
  try {
    role = sessionStorage.getItem('tasklab-role') || '';
    teamID = sessionStorage.getItem('tasklab-selected-team') || '';
  } catch { /* Navigation remains available without browser storage. */ }

  if (role === 'business') {
    roleLink.textContent = 'Роль: Бизнес';
    return;
  }
  if (role !== 'student' || !teamID) {
    roleLink.textContent = 'Роль: не выбрана';
    return;
  }

  fetch('/api/teams')
    .then(response => {
      if (!response.ok) throw new Error('Не удалось загрузить команды');
      return response.json();
    })
    .then(teams => {
      const team = teams.find(item => String(item.id) === teamID);
      roleLink.textContent = team ? `Роль: Команда ${team.name}` : 'Роль: не выбрана';
    })
    .catch(() => { roleLink.textContent = 'Роль: не выбрана'; });
})();
