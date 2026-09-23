'use strict';
const { request, node } = window.TaskLab;
const $ = selector => document.querySelector(selector);
const page = document.body.dataset.page;

if (page === 'login') {
  $('#auth-form').addEventListener('submit', async event => {
    event.preventDefault();
    try {
      const role = event.currentTarget.elements.role.value;
      const button = event.currentTarget.querySelector('button[type=submit]'); button.disabled = true;
      await request('/api/session', { method: 'POST', body: JSON.stringify({ role }) });
      location.href = role === 'business' ? '/static/task-builder/business.html' : '/static/task-builder/teams.html';
    } catch (error) { $('#auth-error').textContent = error.message; $('#auth-form button[type=submit]').disabled = false; }
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
  try { const session = await TaskLab.ready; selected = session.team_id || null; }
  catch (error) { message(error.message); return; }

  async function selectTeam(team) {
    if (busy || selected) return;
    if (!window.confirm(`Вступить в команду «${team.name}»? После подтверждения смена команды недоступна.`)) return;
    busy = true; render();
    try {
      TaskLab.session = await request('/api/session/team', { method:'POST', body:JSON.stringify({team_id:team.id}) });
      selected = TaskLab.session.team_id;
      render();
      message(`Вы состоите в команде «${team.name}».`, true);
      document.dispatchEvent(new CustomEvent('team:selected', {detail:team}));
    } catch (error) { message(error.message); }
    finally { busy = false; render(); }
  }

  function render() {
    $('#open-create').hidden = Boolean(selected);
    $('.team-tools').hidden = Boolean(selected);
    document.querySelector('.page-heading h1').textContent = selected ? 'Ваша команда' : 'Выберите свою команду';
    document.querySelector('.page-heading p').textContent = selected ? 'Ваши навыки, интересы и баллы за подтверждённые этапы.' : 'Вступите в одну команду или создайте свою. После вступления смена команды недоступна.';
    document.querySelector('.demo-note').hidden = Boolean(selected);
    const query = $('#search').value.trim().toLocaleLowerCase('ru');
    const category = $('#category-filter').value;
    const filtered = teams.filter(team => selected ? team.id === selected : (!category || team.interests.includes(category)) && `${team.name} ${team.skills} ${team.tech}`.toLocaleLowerCase('ru').includes(query));
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
      choose.disabled = busy || Boolean(selected);
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
      const catalog = node('a', 'Перейти к задачам →', 'text-link');
      catalog.href = '/';
      panel.append(node('h3', team.name), node('p', `Баллы: ${team.points}`), catalog, node('p', 'Команда закреплена за вами. Смена команды недоступна.', 'hint'));
    }
  }

  async function load() {
    $('#open-create').disabled = true;
    message('Загружаем команды из базы…');
    try {
      TaskLab.session = await request('/api/session'); selected = TaskLab.session.team_id || null;
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
      teams.push(team); selected = team.id; TaskLab.session.team_id = team.id;
      $('#create-dialog').close(); render(); message('Команда создана и закреплена за вами.', true);
      document.dispatchEvent(new CustomEvent('team:selected', {detail:team}));
    } catch (error) { $('#create-error').textContent = error.message; }
    finally { busy = false; button.disabled = false; }
  });
  await load();
}
