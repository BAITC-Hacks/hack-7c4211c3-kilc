'use strict';
const { request, node } = window.TaskLab;
const $ = selector => document.querySelector(selector);
const statusLabels = { pending: 'Ожидает решения', accepted: 'Принято', rejected: 'Отклонено' };
let tasks = [];
let categories = [];
let activeTaskID = null;
let busy = false;

function lock(value) {
  busy = value;
  document.querySelectorAll('main button, main input, main select').forEach(control => { control.disabled = value; });
  $('main').setAttribute('aria-busy', String(value));
}
function report(error) { $('#business-error').textContent = error.message || String(error); }
async function operation(callback) {
  if (busy) return;
  lock(true); $('#business-error').textContent = ''; $('#operation-status').textContent = 'Загружаем…';
  try { await callback(); }
  catch (error) { report(error); }
  finally { lock(false); if ($('#operation-status').textContent === 'Загружаем…') $('#operation-status').textContent = ''; }
}

function renderTasks() {
  const list = $('#task-list'); list.replaceChildren();
  $('#task-count').textContent = `Всего: ${tasks.length}`;
  if (!tasks.length) list.append(node('p', 'Задач этой компании пока нет.', 'empty'));
  for (const task of tasks) {
    const card = node('article', undefined, 'section business-task');
    card.dataset.open = String(task.id === activeTaskID);
    card.classList.add('clickable-card');
    const activate = () => { if (!busy) card.querySelector('.buttons a, .buttons button')?.click(); };
    card.addEventListener('click', event => { if (!event.target.closest('a,button,input,select') && !window.getSelection().toString()) activate(); });
    card.append(node('h3', task.title || `Задача №${task.id}`), node('p', task.company || 'Компания не указана', 'hint'),
      node('p', task.status === 'published' ? 'Опубликована' : 'Черновик', 'badge'),
      node('p', `${task.score} / 100 · ${task.level_label}`),
      node('p', `Откликов: ${task.proposals} (ожидают: ${task.pending}, принято: ${task.accepted}, отклонено: ${task.rejected})`));
    if (task.reward) card.append(node('p', `${task.reward} · Бонус +${task.bonus}`, 'reward-note'));
    const actions = node('div', undefined, 'buttons');
    if (task.status === 'draft') {
      const edit = node('a', 'Доработать', 'text-link'); edit.href = `index.html?id=${task.id}`; actions.append(edit);
    } else {
      const open = node('button', 'Сравнить отклики', 'primary'); open.type = 'button'; open.disabled = busy;
      open.addEventListener('click', () => operation(async () => {
        activeTaskID = task.id; renderTasks();
        await loadPanel();
        $('#panel-title').focus();
        $('#proposals-panel').scrollIntoView({ behavior: 'smooth', block: 'start' });
      })); actions.append(open);
    }
    card.append(actions); list.append(card);
  }
}

async function loadTasks() {
  try {
    tasks = await request('/api/business/tasks');
    if (activeTaskID && !tasks.some(task => task.id === activeTaskID && task.status === 'published')) {
      activeTaskID = null; $('#proposals-panel').hidden = true;
    }
    renderTasks();
  } catch (error) {
    $('#task-list').replaceChildren(node('p', 'Список задач не загружен. Нажмите «Обновить».', 'empty'));
    $('#task-count').textContent = ''; throw error;
  }
}

function addDetail(list, label, value) { list.append(node('dt', label), node('dd', value?.trim() || 'Не указано')); }

function renderProposals(proposals) {
  const list = $('#proposal-list'); list.replaceChildren();
  if (!proposals.length) list.append(node('p', 'На эту задачу пока нет откликов.', 'empty'));
  // Preserve server order; every decision belongs to its individual proposal.
  for (const proposal of proposals) {
    const team = proposal.team;
    const name = team.name || proposal.team_name;
    const card = node('article', undefined, 'section proposal-card');
    card.append(node('h3', name), node('p', `Баллы команды: ${team.points}`, 'hint'),
      node('span', statusLabels[proposal.status] || proposal.status, `badge ${proposal.status}`));
    const details = node('dl');
    addDetail(details, 'Интересы', (team.interests || []).map(code => categories.find(item => item.code === code)?.label || code).join(', '));
    addDetail(details, 'Навыки', team.skills); addDetail(details, 'Технологии', team.tech);
    addDetail(details, 'Идея решения', proposal.idea); addDetail(details, 'План работы', proposal.plan);
    addDetail(details, 'Срок', proposal.deadline);
    details.append(node('dt', 'Прототип'));
    const prototype = node('dd');
    try {
      const url = new URL(proposal.prototype_url);
      if (!['http:', 'https:'].includes(url.protocol)) throw new Error('invalid URL');
      const link = node('a', proposal.prototype_url); link.href = url.href; link.target = '_blank'; link.rel = 'noopener'; prototype.append(link);
    } catch { prototype.textContent = 'Корректная ссылка на прототип не указана'; }
    details.append(prototype); card.append(details);
    if (proposal.status === 'pending') {
      const actions = node('div', undefined, 'buttons');
      for (const [status, label] of [['accepted', 'Выбрать команду'], ['rejected', 'Отклонить']]) {
        const button = node('button', label, status === 'accepted' ? 'primary' : 'secondary'); button.type = 'button'; button.disabled = busy;
        button.addEventListener('click', () => {
          if (busy || !window.confirm(status === 'accepted' ? `Выбрать команду «${name}» для этой задачи?` : `Отклонить предложение команды «${name}»?`)) return;
          changeProposal(proposal.id, 'decision', { status });
        }); actions.append(button);
      }
      card.append(actions);
    } else if (proposal.status === 'accepted') {
      if (proposal.stage_confirmed_at) card.append(node('p', `Этап подтверждён: +${proposal.points_awarded} баллов`, 'stage-done'));
      else {
        const form = node('form', undefined, 'stage-form');
        const field = node('div', undefined, 'field');
        const label = node('label', 'Баллы за этап'); label.htmlFor = `points-${proposal.id}`;
        const input = node('input'); input.type = 'number'; input.id = label.htmlFor;
        input.min = '1'; input.max = '100'; input.step = '1'; input.value = '10'; input.required = true; input.disabled = busy;
        field.append(label, input);
        const button = node('button', 'Подтвердить этап', 'primary'); button.type = 'submit'; button.disabled = busy;
        const actions = node('div', undefined, 'buttons'); actions.append(button); form.append(field, actions);
        form.addEventListener('submit', event => {
          event.preventDefault(); if (busy || !form.reportValidity()) return;
          const points = Number(input.value);
          if (!window.confirm(`Подтвердить этап команды «${name}» и начислить ${points} баллов?`)) return;
          changeProposal(proposal.id, 'stage', { points });
        }); card.append(form);
      }
    }
    list.append(card);
  }
}

async function loadPanel() {
  if (!activeTaskID) return;
  $('#proposals-panel').hidden = false;
  const task = tasks.find(item => item.id === activeTaskID);
  $('#panel-title').textContent = task?.title || `Задача №${activeTaskID}`;
  $('#proposal-list').replaceChildren(node('p', 'Загружаем предложения…', 'hint'));
  try { renderProposals(await request(`/api/tasks/${activeTaskID}/proposals/full`)); }
  catch (error) {
    $('#proposal-list').replaceChildren(node('p', 'Предложения не загружены. Нажмите «Обновить».', 'empty'));
    throw error;
  }
}

async function changeProposal(id, action, body) {
  await operation(async () => {
    let actionError;
    try {
      await request(`/api/proposals/${id}/${action}`, { method: 'POST', body: JSON.stringify(body) });
      $('#operation-status').textContent = action === 'stage' ? 'Этап подтверждён.' : 'Решение сохранено.';
    } catch (error) { actionError = error; $('#operation-status').textContent = ''; }
    // Refresh even after a conflict or timeout so stale actions are not offered.
    const results = await Promise.allSettled([loadTasks(), loadPanel()]);
    const failures = results.filter(result => result.status === 'rejected').map(result => result.reason.message);
    if (actionError) failures.unshift(actionError.message);
    if (failures.length) report(new Error(failures.join(' ')));
  });
}

async function initialise() {
  const session = await TaskLab.ready;
  $('#company-name').textContent = session.company;
  categories = await request('/api/categories');
  await loadTasks(); await loadPanel();
}
$('#refresh').addEventListener('click', () => operation(initialise));
operation(initialise);
