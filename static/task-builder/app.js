'use strict';

// Поля можно переиспользовать при подключении API или AI-помощника.
const fields = [
  { id: 'draft_text', group: 'basics', label: 'Опишите исходную задачу', placeholder: 'Расскажите о своей потребности своими словами…', hint: 'После первого сохранения исходное описание сохраняется без изменений.', max: 4000, required: true },
  { id: 'company', group: 'basics', label: 'Компания', placeholder: 'Название компании', hint: 'Кто предлагает задачу?', type: 'text', max: 250 },
  { id: 'industry', group: 'basics', label: 'Отрасль', placeholder: 'Например: розничная торговля', hint: 'Используется для фильтрации каталога.', type: 'text', max: 250 },
  { id: 'title', group: 'basics', label: 'Как называется ваша задача?', placeholder: 'Например: автоматизация обработки заявок', hint: 'Короткое и понятное название.', type: 'text', max: 120 },
  { id: 'category', group: 'basics', label: 'Категория задачи', type: 'select', hint: 'Категория из справочника сервера.' },
  { id: 'context', group: 'basics', label: 'Как процесс устроен сейчас?', placeholder: 'Заявки приходят по почте. Менеджер вручную переносит их в таблицу…', hint: 'Опишите текущую ситуацию.', max: 2000 },
  { id: 'need', group: 'basics', label: 'Какую проблему нужно решить?', placeholder: 'Хотим уменьшить ручной ввод и количество потерянных заявок…', hint: 'Что мешает бизнесу и что необходимо изменить?', max: 2000 },
  { id: 'users', group: 'basics', label: 'Кто будет пользоваться решением?', placeholder: 'Три менеджера отдела продаж и руководитель…', hint: 'Назовите пользователей и их задачи.', max: 1000 },
  { id: 'data', group: 'details', label: 'Какие данные и материалы доступны?', placeholder: 'Обезличенная таблица заявок за месяц, примеры писем…', hint: 'Укажите формат, примеры или источники. Если данных нет — так и напишите.', max: 2000 },
  { id: 'expected_result', group: 'details', label: 'Что вы хотите получить в результате?', placeholder: 'Веб-прототип, который собирает заявки в единый список…', hint: 'Опишите конкретный результат работы команды.', max: 2000 },
  { id: 'success_criteria', group: 'details', label: 'Как вы поймёте, что задача решена?', placeholder: 'Обработка тестовой заявки занимает не более 1 минуты…', hint: 'Добавьте измеримые критерии приёмки.', max: 2000 },
  { id: 'constraints', group: 'details', label: 'Какие есть ограничения?', placeholder: 'Прототип за 2 недели; только обезличенные данные…', hint: 'Сроки, технологии, доступы, бюджет или другие границы.', max: 2000 },
  { id: 'contact', group: 'contact', label: 'Как с вами связаться?', placeholder: 'Имя и рабочая почта или Telegram', hint: 'Контакт представителя бизнеса для команды.', type: 'text', max: 250 },
  { id: 'interaction_format', group: 'contact', label: 'Как будет организована обратная связь?', placeholder: 'Созвон по вторникам, ответы на вопросы в течение рабочего дня…', hint: 'Укажите формат консультаций и порядок обратной связи.', max: 1000 }
];

const { request, node } = window.TaskLab;
const form = document.querySelector('#task-form');
const $ = selector => document.querySelector(selector);
const ratingFields = ['context', 'need', 'users', 'data', 'constraints', 'expected_result', 'success_criteria', 'contact', 'interaction_format'];
const confirmableFields = [...ratingFields, 'reward'];
const confirmed = new Set();
const DRAFT_KEY = 'tasklab-server-draft-v1';
let taskId = null;
let qa = [];
let currentTask = null;
let revision = 0;
let timer;
let busy = false;

for (const field of fields) {
  const wrapper = node('div', undefined, 'field');
  const label = node('label', field.label + (field.required ? ' *' : ''));
  label.htmlFor = field.id;
  const input = node(field.type === 'select' ? 'select' : field.type === 'text' ? 'input' : 'textarea');
  input.id = field.id; input.name = field.id; input.required = Boolean(field.required);
  if (field.max) input.maxLength = field.max;
  if (field.placeholder) input.placeholder = field.placeholder;
  input.setAttribute('aria-describedby', `${field.id}-hint`);
  const hint = node('p', field.hint, 'hint'); hint.id = `${field.id}-hint`;
  wrapper.append(label, input, hint);
  if (ratingFields.includes(field.id)) {
    const checkLabel = node('label', undefined, 'confirmation');
    checkLabel.style.marginTop = '10px';
    const check = node('input'); check.type = 'checkbox'; check.dataset.confirm = field.id;
    check.style.width = '18px'; check.style.padding = '0'; check.style.boxShadow = 'none';
    checkLabel.append(check, node('span', 'Сведения проверены')); wrapper.append(checkLabel);
  }
  $(`#${field.group}-fields`).append(wrapper);
}

function values() { return { ...Object.fromEntries(fields.map(field => [field.id, form.elements[field.id].value.trim()])), reward: form.elements.reward.value.trim(), reward_type: form.elements.reward_type.value }; }
function payload() { const data = values(); return { ...data, reward_type: data.reward_type, reward: data.reward, qa, confirmed: [...confirmed] }; }
function setStatus(text) { $('#save-status').textContent = text; }
function lock(locked) {
  busy = locked;
  for (const control of form.elements) control.disabled = locked;
  form.elements.draft_text.readOnly = Boolean(taskId);
  form.elements.company.readOnly = true;
}
function updateChecks() {
  const data = values();
  for (const check of form.querySelectorAll('[data-confirm]')) {
    check.checked = confirmed.has(check.dataset.confirm);
    check.disabled = busy || !data[check.dataset.confirm];
  }
  const filled = confirmableFields.filter(key => data[key]);
  $('#confirmed').checked = filled.length > 0 && filled.every(key => confirmed.has(key));
  $('#confirmed').indeterminate = !$('#confirmed').checked && filled.some(key => confirmed.has(key));
}
function showRating(result) {
  $('#score').textContent = result.total; $('#progress').value = result.total;
  $('#level').textContent = result.level_label;
  $('#score-state').textContent = `Потенциал: ${result.potential}/100`;
  $('#breakdown').replaceChildren();
  for (const item of result.components) {
    const row = node('div', undefined, `criterion${item.score === item.max ? ' complete' : ''}`);
    row.append(node('span', item.label), node('strong', `${item.score} / ${item.max}`));
    $('#breakdown').append(row);
  }
  $('#next-tip').replaceChildren();
  if (!result.hints.length) $('#next-tip').textContent = 'Все поля заполнены и подтверждены.';
  for (const hint of result.hints) $('#next-tip').append(node('span', hint.message), node('br'));
  $('#bonus').textContent = `+${result.bonus ?? 0}`;
  $('#position').textContent = result.position ?? result.total;
  $('#bonus-hint').textContent = result.bonus_hint || '';
  $('#bonus-hint').hidden = !result.bonus_hint;
}
function invalidateRating() {
  revision++; clearTimeout(timer);
  $('#score').textContent = '…'; $('#progress').value = 0; $('#level').textContent = 'Пересчёт';
  $('#score-state').textContent = 'Запрос к серверу'; $('#breakdown').replaceChildren();
  $('#bonus').textContent = '+0'; $('#position').textContent = '—'; $('#bonus-hint').textContent = ''; $('#bonus-hint').hidden = true;
  timer = setTimeout(refreshRating, 300);
}
async function refreshRating() {
  const version = revision;
  const data = values();
  const card = Object.fromEntries(ratingFields.map(key => [key, { text: data[key], confirmed: confirmed.has(key) }]));
  card.reward_type = data.reward_type;
  card.reward = { text: data.reward, confirmed: confirmed.has('reward') };
  try {
    const result = await request('/api/rating', { method: 'POST', body: JSON.stringify(card) });
    if (version === revision) showRating(result);
  } catch (error) {
    if (version !== revision) return;
    $('#score').textContent = '—'; $('#level').textContent = 'Нет оценки';
    $('#score-state').textContent = 'API недоступен'; $('#next-tip').textContent = error.message;
  }
}
function saveLocalDraft() {
  if (taskId) return;
  try { localStorage.setItem(DRAFT_KEY, JSON.stringify(values())); setStatus('Локальная копия сохранена. Для записи в базу нажмите «Сохранить задачу».'); }
  catch { setStatus('Локальная копия не сохранена. Можно сохранить задачу непосредственно на сервере.'); }
}
form.addEventListener('input', event => {
  if (busy) return;
  const target = event.target;
  if (target.dataset.confirm) {
    if (target.checked && values()[target.dataset.confirm]) confirmed.add(target.dataset.confirm);
    else confirmed.delete(target.dataset.confirm);
  } else if (target.id === 'confirmed') {
    confirmed.clear();
    if (target.checked) for (const key of confirmableFields) if (values()[key]) confirmed.add(key);
  } else {
    confirmed.delete(target.id === 'reward_type' ? 'reward' : target.name); saveLocalDraft();
  }
  currentTask = null; $('#preview').hidden = true;
  if (taskId) setStatus('Есть несохранённые изменения.');
  updateChecks(); invalidateRating();
});
function preview(task) {
  currentTask = task;
  const container = $('#preview-content'); container.replaceChildren();
  container.append(node('p', `Задача №${task.id} · ${task.status === 'published' ? 'Опубликована' : 'Черновик'} · ${task.rating.total}/100`));
  const list = node('dl');
  for (const field of fields) {
    let text = task[field.id] || 'Не указано';
    if (field.id === 'category') text = form.elements.category.selectedOptions[0]?.textContent || text;
    list.append(node('dt', field.label), node('dd', text));
  }
  list.append(node('dt', 'Вознаграждение'), node('dd', task.reward || 'Не предусмотрено'));
  container.append(list); $('#preview').hidden = false; showRating(task.rating);
}
async function save(publish) {
  if (busy || !form.reportValidity()) return;
  const body = payload();
  if (!body.draft_text) { setStatus('Введите исходное описание задачи.'); form.elements.draft_text.focus(); return; }
  if (publish && (!body.title || confirmableFields.some(key => body[key] && !confirmed.has(key)))) {
    setStatus('Для публикации укажите название и подтвердите все заполненные поля.'); return;
  }
  clearTimeout(timer); revision++; lock(true); setStatus('Сохраняем в базе…');
  try {
    let task = await request(taskId ? `/api/tasks/${taskId}` : '/api/tasks', { method: taskId ? 'PUT' : 'POST', body: JSON.stringify(body) });
    taskId = task.id;
    history.replaceState(null, '', `?id=${taskId}`);
    let localWarning = '';
    try { localStorage.removeItem(DRAFT_KEY); } catch { localWarning = ' Локальную копию удалить не удалось.'; }
    preview(task);
    if (publish) {
      setStatus('Задача сохранена. Публикуем…');
      try {
        task = await request(`/api/tasks/${taskId}/publish`, { method: 'POST', body: '{}' });
        preview(task);
      } catch (error) { setStatus(`Задача №${taskId} сохранена, но публикация не выполнена: ${error.message}`); return; }
    }
    setStatus(`Задача №${taskId} ${task.status === 'published' ? 'опубликована' : 'сохранена'} в базе.${localWarning}`);
    form.dispatchEvent(new CustomEvent('task:saved', { bubbles: true, detail: task }));
  } catch (error) { setStatus(error.message); }
  finally { lock(false); updateChecks(); }
}
form.addEventListener('submit', event => { event.preventDefault(); save(false); });
$('#publish').addEventListener('click', () => save(true));
$('#reset').textContent = 'Новая задача';
$('#reset').addEventListener('click', () => {
  if (!confirm('Начать новую задачу? Несохранённые изменения будут потеряны. Записи в базе останутся.')) return;
  form.reset(); form.elements.company.value = TaskLab.session.company; taskId = null; qa = []; confirmed.clear(); currentTask = null;
  history.replaceState(null, '', location.pathname); form.elements.draft_text.readOnly = false;
  $('#preview').hidden = true; saveLocalDraft(); updateChecks(); invalidateRating();
});
$('#download').addEventListener('click', () => {
  if (!currentTask) return;
  const url = URL.createObjectURL(new Blob([JSON.stringify(currentTask, null, 2)], { type: 'application/json' }));
  const link = node('a'); link.href = url; link.download = `task-${currentTask.id}.json`; link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
});
async function init() {
  lock(true);
  const requestedID = new URLSearchParams(location.search).get('id');
  try {
    await TaskLab.ready;
    const categories = await request('/api/categories');
    form.elements.category.append(new Option('Не выбрана', ''));
    for (const category of categories) form.elements.category.append(new Option(category.label, category.code));
    const rewardTypes = await request('/api/reward-types');
    form.elements.reward_type.replaceChildren();
    for (const rewardType of rewardTypes) form.elements.reward_type.append(new Option(rewardType.label, rewardType.code));
    if (requestedID) {
      if (!/^[1-9]\d*$/.test(requestedID)) throw new Error('Некорректный ID задачи в адресе.');
      const task = await request(`/api/tasks/${requestedID}`);
      taskId = task.id; qa = task.qa || [];
      for (const field of fields) form.elements[field.id].value = task[field.id] || '';
      form.elements.reward.value = task.reward || '';
      form.elements.reward_type.value = task.reward_type || '';
      for (const key of task.confirmed) confirmed.add(key);
      preview(task); setStatus(`Загружена задача №${taskId}.`);
    } else {
      form.elements.reward_type.value = '';
      try {
        const draft = JSON.parse(localStorage.getItem(DRAFT_KEY) || 'null');
        if (draft && typeof draft === 'object') for (const field of fields) {
          if (typeof draft[field.id] === 'string') form.elements[field.id].value = draft[field.id].slice(0, field.max || 100);
        }
        if (draft && typeof draft.reward === 'string') form.elements.reward.value = draft.reward.slice(0, 300);
        if (draft && typeof draft.reward_type === 'string') form.elements.reward_type.value = draft.reward_type;
      } catch { setStatus('Не удалось восстановить локальный черновик. Заполните поля заново.'); }
    }
    form.elements.company.value = TaskLab.session.company;
    lock(false); updateChecks(); await refreshRating();
  } catch (error) {
    setStatus(`${error.message} После исправления обновите страницу.`);
    $('#score').textContent = '—'; $('#level').textContent = 'Нет соединения';
  }
}
init();
