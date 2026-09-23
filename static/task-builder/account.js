'use strict';
const { request, node } = window.TaskLab;
const $ = selector => document.querySelector(selector);
const ROLE_KEY = 'tasklab-role';
const TEAM_KEY = 'tasklab-selected-team';
const page = document.body.dataset.page;

function saveSelection(key, value) {
  try { sessionStorage.setItem(key, String(value)); }
  catch { throw new Error('Не удалось сохранить выбранную роль или команду в браузере.'); }
}

if (page === 'login') {
  $('#auth-form').addEventListener('submit', event => {
    event.preventDefault();
    try {
      const role = event.currentTarget.elements.role.value;
      saveSelection(ROLE_KEY, role);
      location.href = role === 'business' ? '/static/task-builder/business.html' : '/static/task-builder/teams.html';
    } catch (error) { $('#auth-error').textContent = error.message; }
  });
}

if (page === 'teams') initTeams();

async function initTeams() {
  let teams = [];
  let categories = [];
  let selected = null;
  let busy = false;
  const message = (text, success = false) => {
    $('#team-message').textContent = text;
    $('#team-message').classList.toggle('success', success);
  };
  try { selected = Number(sessionStorage.getItem(TEAM_KEY)) || null; }
  catch { message('Выбранная команда недоступна в памяти браузера. Выберите её повторно.'); }

  function selectTeam(team) {
    try {
      saveSelection(ROLE_KEY, 'student');
      saveSelection(TEAM_KEY, team.id);
      selected = team.id;
      render();
      message(`Выбрана команда «${team.name}». Это выбор роли для работы в MVP, без учёта участников.`, true);
      document.dispatchEvent(new CustomEvent('team:selected', { detail: team }));
    } catch (error) { message(error.message); }
  }

  function render() {
    const query = $('#search').value.trim().toLocaleLowerCase('ru');
    const category = $('#category-filter').value;
    const filtered = teams.filter(team => (!category || team.interests.includes(category))
      && `${team.name} ${team.skills} ${team.tech}`.toLocaleLowerCase('ru').includes(query));
    const grid = $('#team-grid');
    grid.replaceChildren();
    $('#team-count').textContent = `Команд: ${filtered.length}`;
    if (!filtered.length) grid.append(node('p', 'Нет подходящих команд. Измените фильтры или создайте команду.', 'empty'));
    for (const team of filtered) {
      const card = node('article', undefined, 'section team-card');
      const tags = node('div', undefined, 'tags');
      for (const code of team.interests) tags.append(node('span', categories.find(c => c.code === code)?.label || code, 'tag'));
      const choose = node('button', selected === team.id ? 'Команда выбрана' : 'Выбрать команду', 'primary');
      choose.type = 'button';
      choose.disabled = selected === team.id;
      choose.addEventListener('click', () => selectTeam(team));
      card.append(node('div', team.name.slice(0, 2).toUpperCase(), 'team-icon'), node('h3', team.name),
        node('p', `Навыки: ${team.skills || 'Не указаны'}`), node('p', `Технологии: ${team.tech || 'Не указаны'}`),
        tags, node('div', `Баллы за подтверждённые этапы: ${team.points}`, 'team-meta'), choose);
      grid.append(card);
    }
    const team = teams.find(team => team.id === selected);
    const panel = $('#my-team');
    panel.replaceChildren();
    if (!team) panel.append(node('p', 'Выберите команду из базы или создайте новую.'));
    else {
      const clear = node('button', 'Сбросить выбор', 'secondary');
      clear.type = 'button';
      clear.addEventListener('click', () => {
        try { sessionStorage.removeItem(TEAM_KEY); selected = null; render(); document.dispatchEvent(new CustomEvent('team:selected')); }
        catch { message('Не удалось сбросить выбор команды.'); }
      });
      const catalog = node('a', 'Перейти к задачам →', 'text-link');
      catalog.href = '/';
      panel.append(node('h3', team.name), node('p', `Баллы: ${team.points}`), catalog, node('p'), clear);
    }
  }

  async function load() {
    $('#open-create').disabled = true;
    message('Загружаем команды из базы…');
    try {
      [teams, categories] = await Promise.all([request('/api/teams'), request('/api/categories')]);
      const filter = $('#category-filter');
      filter.replaceChildren(new Option('Все интересы', ''));
      const interests = $('#team-interests');
      interests.replaceChildren();
      for (const item of categories) {
        filter.append(new Option(item.label, item.code));
        const label = node('label', undefined, 'confirmation');
        const input = node('input'); input.type = 'checkbox'; input.name = 'interests'; input.value = item.code;
        label.append(input, node('span', item.label)); interests.append(label);
      }
      render(); message(''); $('#open-create').disabled = false;
    } catch (error) { message(error.message); $('#team-count').textContent = 'Каталог не загружен'; }
  }

  $('#search').addEventListener('input', render);
  $('#category-filter').addEventListener('change', render);
  $('#refresh-teams').addEventListener('click', load);
  $('#open-create').addEventListener('click', () => {
    $('#create-team-form').reset(); $('#create-error').textContent = ''; $('#create-dialog').showModal();
  });
  $('#close-create').addEventListener('click', () => { if (!busy) $('#create-dialog').close(); });
  $('#create-dialog').addEventListener('cancel', event => { if (busy) event.preventDefault(); });
  $('#create-team-form').addEventListener('submit', async event => {
    event.preventDefault(); if (busy) return;
    const form = event.currentTarget;
    const body = { name: form.elements.name.value.trim(), skills: form.elements.skills.value.trim(),
      tech: form.elements.tech.value.trim(), interests: [...form.querySelectorAll('[name=interests]:checked')].map(input => input.value) };
    if (!body.name) { $('#create-error').textContent = 'Введите название команды.'; return; }
    busy = true; const button = form.querySelector('[type=submit]'); button.disabled = true;
    $('#create-error').textContent = 'Сохраняем команду…';
    try {
      const team = await request('/api/teams', { method: 'POST', body: JSON.stringify(body) });
      teams.push(team); $('#create-dialog').close(); selectTeam(team);
    } catch (error) { $('#create-error').textContent = error.message; }
    finally { busy = false; button.disabled = false; }
  });
  await load();
}
