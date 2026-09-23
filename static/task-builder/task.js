'use strict';
const { request, node } = window.TaskLab;
const $ = selector => document.querySelector(selector);
const form = $('#proposal-form');
const taskID = new URLSearchParams(location.search).get('id');
const proposalFields = ['team_id', 'idea', 'plan', 'deadline', 'prototype_url'];
let sending = false;

// TaskLab.request currently drops HTTP status and field metadata on errors.
// Keep them locally for this form without changing the shared client contract.
async function detailedRequest(path, options = {}) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 12000);
  try {
    const response = await fetch(path, { ...options, headers: { 'Content-Type': 'application/json' }, signal: controller.signal });
    let data;
    try { data = await response.json(); }
    catch { throw new Error('Сервер вернул некорректный ответ.'); }
    if (!response.ok) {
      const error = new Error(data.error || `Ошибка сервера: ${response.status}`);
      error.status = response.status; error.field = data.field; throw error;
    }
    return data;
  } catch (error) {
    if (error.name === 'AbortError') throw new Error('Сервер не ответил вовремя. Проверьте число откликов перед повторной отправкой.');
    if (error instanceof TypeError) throw new Error('Не удалось связаться с сервером. Проверьте соединение.');
    throw error;
  } finally { clearTimeout(timeout); }
}

async function refreshCount() {
  try {
    const proposals = await request(`/api/tasks/${taskID}/proposals`);
    $('#proposal-count').textContent = `Откликов: ${proposals.length}`;
    $('#count-error').textContent = '';
  } catch (error) {
    $('#proposal-count').textContent = 'Откликов: —';
    $('#count-error').textContent = `Не удалось обновить число откликов: ${error.message}`;
  }
}

function clearErrors() {
  for (const key of ['task_id', ...proposalFields]) $(`#error-${key}`).textContent = '';
  for (const key of proposalFields) form.elements[key].removeAttribute('aria-invalid');
  $('#proposal-message').textContent = '';
  $('#proposal-message').classList.remove('success');
}

form.addEventListener('input', event => {
  const key = event.target.name;
  if (proposalFields.includes(key)) {
    $(`#error-${key}`).textContent = ''; event.target.removeAttribute('aria-invalid');
  }
});
form.addEventListener('submit', async event => {
  event.preventDefault(); if (sending || !form.reportValidity()) return;
  clearErrors();
  const body = Object.fromEntries(proposalFields.map(key => [key, form.elements[key].value]));
  body.team_id = Number(body.team_id);
  sending = true;
  for (const control of form.elements) control.disabled = true;
  $('#submit-proposal').textContent = 'Отправляем…';
  try {
    await detailedRequest(`/api/tasks/${taskID}/proposals`, { method: 'POST', body: JSON.stringify(body) });
    form.reset();
    form.elements.team_id.value = String(TaskLab.session.team_id);
    $('#proposal-message').textContent = 'Предложение отправлено. Решение принимает представитель бизнеса.';
    $('#proposal-message').classList.add('success');
    await refreshCount();
  } catch (error) {
    if (error.status === 400 && ['task_id', ...proposalFields].includes(error.field)) {
      $(`#error-${error.field}`).textContent = error.message;
      if (form.elements[error.field]) form.elements[error.field].setAttribute('aria-invalid', 'true');
    } else $('#proposal-message').textContent = error.message;
  } finally {
    sending = false;
    for (const control of form.elements) control.disabled = false;
    form.elements.team_id.disabled = true;
    $('#submit-proposal').textContent = 'Отправить предложение';
    form.querySelector('[aria-invalid=true]')?.focus();
  }
});

async function init() {
  if (!taskID || !/^[1-9]\d*$/.test(taskID)) {
    $('#page-message').textContent = 'Задача не найдена или не опубликована'; return;
  }
  try {
    await TaskLab.ready;
    const task = await detailedRequest(`/api/tasks/${taskID}`);
    if (task.status !== 'published') {
      $('#page-message').textContent = 'Задача не найдена или не опубликована'; return;
    }
    const categories = await request('/api/categories');
    document.title = `${task.title} — TaskLab`;
    $('#task-title').textContent = task.title;
    $('#task-meta').textContent = [task.company, task.industry, categories.find(c => c.code === task.category)?.label].filter(Boolean).join(' · ');
    $('#task-score').textContent = task.rating.total;
    $('#task-level').textContent = task.rating.level_label;
    if (task.rating.bonus) $('#task-level').after(node('p', `Бонус за вознаграждение: +${task.rating.bonus} · Приоритет в каталоге: ${task.rating.position}`, 'reward-note'));
    $('#draft-note').hidden = task.rating.level !== 'draft';
    const fields = [ ['context', 'Контекст'], ['need', 'Потребность'], ['users', 'Пользователи'], ['data', 'Данные и материалы'],
      ['constraints', 'Ограничения'], ['expected_result', 'Ожидаемый результат'], ['success_criteria', 'Критерии успеха'],
      ['contact', 'Контакт'], ['interaction_format', 'Формат взаимодействия'], ['reward', 'Вознаграждение'] ];
    for (const [key, label] of fields) $('#task-fields').append(node('dt', label), node('dd', task[key]?.trim() || 'Не указано'));
    for (const part of task.rating.components) {
      const row = node('div', undefined, 'criterion');
      row.append(node('span', part.label), node('strong', `${part.score} / ${part.max}`)); $('#task-components').append(row);
    }
    for (const hint of task.rating.hints) $('#task-hints').append(node('li', hint.message));
    if (!task.rating.hints.length) $('#task-hints').append(node('li', 'Все сведения заполнены и подтверждены.'));
    $('#task-page').hidden = false; $('#page-message').textContent = '';
    await Promise.all([refreshCount(), loadTeams()]);
  } catch (error) {
    $('#page-message').textContent = error.status === 404 ? 'Задача не найдена или не опубликована' : error.message;
  }
}

async function loadTeams() {
  if (TaskLab.session.role !== 'student') return;
  if (!TaskLab.session.team_id) {
    const link = node('a', 'Вступить в команду, чтобы отправить предложение →', 'text-link');
    link.href = 'teams.html'; $('#page-message').append(link); return;
  }
  try {
    const teams = await request('/api/teams');
    const team = teams.find(t => t.id === TaskLab.session.team_id);
    if (!team) throw new Error('Ваша команда недоступна. Обратитесь к организатору.');
    form.elements.team_id.replaceChildren(new Option(team.name, String(team.id), true, true));
    form.elements.team_id.disabled = true;
    $('#team-note').textContent = 'Предложение будет отправлено от вашей команды.';
    $('#proposal-section').hidden = false;
  } catch (error) { $('#page-message').textContent = error.message; }
}
init();
