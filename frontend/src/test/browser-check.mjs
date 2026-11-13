// Node 22+ and a separately launched Chrome with remote debugging are required.
// Run only against a NEW, isolated memory/fallback backend (this test uses reset).
// AI_BUDDY_ISOLATED_TEST=1 BASE_URL=http://127.0.0.1:4174 CDP_URL=http://127.0.0.1:19223 node src/test/browser-check.mjs
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'

assert.equal(process.env.AI_BUDDY_ISOLATED_TEST, '1', 'Explicit isolated demo acknowledgement is required')
const base = process.env.BASE_URL || 'http://127.0.0.1:5173'
assert.ok(['127.0.0.1', 'localhost'].includes(new URL(base).hostname), 'Local servers only')
const debug = process.env.CDP_URL || 'http://127.0.0.1:19222'
const reportDir = path.resolve('coverage/browser')
await fs.mkdir(reportDir, { recursive: true })
const health = await (await fetch(base + '/api/v1/health')).json()
assert.equal(health.storage_mode, 'memory')
assert.equal(health.openrouter_configured, false)
const initial = await (await fetch(base + '/api/v1/context')).json()
assert.equal(initial.corporate_requests.length, 0, 'Start a fresh isolated backend before this test')
assert.deepEqual(await (await fetch(base + '/api/v1/chat/messages')).json(), [])
const target = (await (await fetch(debug + '/json/list')).json()).find(item => item.type === 'page')
const socket = new WebSocket(target.webSocketDebuggerUrl)
await new Promise((resolve, reject) => { socket.addEventListener('open', resolve, { once: true }); socket.addEventListener('error', reject, { once: true }) })
let sequence = 0, acceptReset = false, expectedErrors = false
const calls = new Map(), errors = [], requests = [], results = [], layouts = []
socket.addEventListener('message', event => {
  const data = JSON.parse(event.data)
  if (data.id) {
    const call = calls.get(data.id)
    if (call) { calls.delete(data.id); data.error ? call.reject(new Error(JSON.stringify(data.error))) : call.resolve(data.result) }
  } else if (data.method === 'Runtime.exceptionThrown' && !expectedErrors) errors.push(data.params.exceptionDetails)
  else if (data.method === 'Runtime.consoleAPICalled' && data.params.type === 'error' && !expectedErrors) errors.push(data.params.args.map(item => item.value || item.description))
  else if (data.method === 'Network.requestWillBeSent') requests.push(data.params.request.url)
  else if (data.method === 'Page.javascriptDialogOpening' && acceptReset) void cdp('Page.handleJavaScriptDialog', { accept: true })
})
function cdp(method, params = {}) {
  return new Promise((resolve, reject) => {
    const id = ++sequence
    calls.set(id, { resolve, reject })
    socket.send(JSON.stringify({ id, method, params }))
  })
}
async function evaluate(expression) {
  const result = await cdp('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true, userGesture: true })
  if (result.exceptionDetails) throw new Error(JSON.stringify(result.exceptionDetails))
  return result.result.value
}
async function wait(expression, timeout = 15000) {
  const start = Date.now()
  while (Date.now() - start < timeout) {
    if (await evaluate(expression)) return
    await new Promise(resolve => setTimeout(resolve, 80))
  }
  throw new Error('Timeout: ' + expression + '\n' + await evaluate('document.body.innerText'))
}
const api = endpoint => evaluate(`fetch('/api/v1${endpoint}').then(response=>response.json())`)
async function clickExpression(expression) {
  const box = await evaluate(`(()=>{const node=${expression};if(!node || node.disabled) throw Error('Missing/disabled click target');node.scrollIntoView({block:'center',behavior:'instant'});const r=node.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()`)
  for (const type of ['mousePressed', 'mouseReleased']) await cdp('Input.dispatchMouseEvent', { type, ...box, button: 'left', clickCount: 1 })
}
const click = text => clickExpression(`[...document.querySelectorAll('button')].find(node=>node.textContent.trim()===${JSON.stringify(text)} || node.querySelector('strong')?.textContent===${JSON.stringify(text)})`)
async function scenario(text) {
  const count = await evaluate("document.querySelectorAll('.message').length")
  await click(text)
  await wait(`document.querySelectorAll('.message').length===${count + 2} && !document.querySelector('#chat-message').disabled`)
}
async function fill(selector, text) {
  await evaluate(`document.querySelector(${JSON.stringify(selector)}).focus();document.querySelector(${JSON.stringify(selector)}).select()`)
  await cdp('Input.insertText', { text })
}
async function ready(route) {
  const heading = { '/': 'Привет,', '/chat': 'ИИ-помощник', '/onboarding': 'Моя адаптация', '/knowledge': 'База знаний', '/requests': 'Заявки' }[route]
  await wait(`location.pathname===${JSON.stringify(route)} && document.querySelector('h1')?.textContent.includes(${JSON.stringify(heading)}) && !document.querySelector('[aria-label="Загрузка раздела"]') && !!document.querySelector('.header-avatar')?.title && !document.querySelector('.app-shell[aria-busy="true"]')`)
  if (route === '/chat') await wait("!!document.querySelector('#chat-message') && !document.querySelector('#chat-message').disabled")
  if (route === '/knowledge') await wait("!!document.querySelector('.knowledge-card') || document.body.innerText.includes('Ничего не найдено')")
}
async function go(route) {
  await evaluate('window.__navigationMarker=true')
  await cdp('Page.navigate', { url: base + route })
  await wait("!window.__navigationMarker && document.readyState==='complete'")
  await ready(route.split('?')[0])
}
async function reload(route) {
  await evaluate('window.__navigationMarker=true')
  await cdp('Page.reload')
  await wait("!window.__navigationMarker && document.readyState==='complete'")
  await ready(route)
}
async function nav(route) { await clickExpression(`document.querySelector(${JSON.stringify(`.sidebar a[href="${route}"]`)})`); await ready(route) }
async function role(value) {
  if (await evaluate("!!document.querySelector('select[aria-label=\"Демонстрационная роль\"]')")) {
    await evaluate(`(()=>{const node=document.querySelector('select[aria-label="Демонстрационная роль"]');node.value=${JSON.stringify(value)};node.dispatchEvent(new Event('change',{bubbles:true}))})()`)
    await wait(`!document.querySelector('.app-shell[aria-busy="true"]') && document.querySelector('select[aria-label="Демонстрационная роль"]').value===${JSON.stringify(value)}`)
  } else {
    // Only the isolated test driver changes shared demo state. Production UI
    // must not expose a role chooser or pretend to authenticate the employee.
    const response = await evaluate(`fetch('/api/v1/auth/switch-role',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({role:${JSON.stringify(value)}})}).then(async response=>({status:response.status,body:await response.json()}))`)
    assert.equal(response.status, 200); assert.equal(response.body.active_role, value)
    await reload(await evaluate('location.pathname'))
  }
}
async function screenshot(name) {
  const shot = await cdp('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false })
  await fs.writeFile(path.join(reportDir, name + '.png'), Buffer.from(shot.data, 'base64'))
}
async function accessibility() {
  return evaluate(`(${function () {
    const rgb = value => (value.match(/[\d.]+/g) || []).map(Number)
    const luminance = color => color.slice(0, 3).map(value => {
      value /= 255
      return value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4
    }).reduce((sum, value, index) => sum + value * [.2126, .7152, .0722][index], 0)
    const contrast = []
    for (const node of document.querySelectorAll('body *')) {
      const style = getComputedStyle(node), rect = node.getBoundingClientRect()
      if (!rect.width || !rect.height || node.classList.contains('sr-only') || style.visibility === 'hidden' || node.closest(':disabled,[aria-hidden="true"]') || ![...node.childNodes].some(child => child.nodeType === 3 && child.textContent.trim())) continue
      let parent = node, background
      while (parent) {
        background = rgb(getComputedStyle(parent).backgroundColor)
        if (background.length === 3 || background[3] === 1) break
        parent = parent.parentElement
      }
      if (!parent) background = [255, 255, 255]
      const foreground = luminance(rgb(style.color)), behind = luminance(background)
      const ratio = (Math.max(foreground, behind) + .05) / (Math.min(foreground, behind) + .05)
      const large = parseFloat(style.fontSize) >= 24 || (parseFloat(style.fontSize) >= 18.66 && parseFloat(style.fontWeight) >= 700)
      if (ratio < (large ? 3 : 4.5)) contrast.push({ text: node.textContent.trim().slice(0, 60), selector: node.className, ratio: +ratio.toFixed(2) })
    }
    const smallTargets = [...document.querySelectorAll('button,select,summary,a.text-link,.citations a')].filter(node => {
      const rect = node.getBoundingClientRect()
      // Chrome can report 43.999... for a 44px box after fractional scrolling.
      return getComputedStyle(node).visibility !== 'hidden' && rect.width && !node.disabled && (rect.width < 43.9 || rect.height < 43.9)
    }).map(node => ({ text: node.textContent.trim().slice(0, 45), selector: node.className, width: node.getBoundingClientRect().width, height: node.getBoundingClientRect().height }))
    return { contrast, smallTargets }
  }.toString()})()`)
}
try {
  await cdp('Runtime.discardConsoleEntries')
  await cdp('Runtime.enable'); await cdp('Network.enable'); await cdp('Page.enable')
  await cdp('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] })
  await cdp('Emulation.setDeviceMetricsOverride', { width: 1440, height: 900, deviceScaleFactor: 1, mobile: false })
  await go('/')
  const production = !await evaluate("!!document.querySelector('select[aria-label=\"Демонстрационная роль\"]')")
  if (production) assert.ok(await evaluate("!document.querySelector('.reset-button') && !document.querySelector('.demo-notice') && !document.body.innerText.includes('Hack&Change 2026')"))
  const firstAssets = await evaluate("performance.getEntriesByType('resource').filter(item=>item.name.includes('/assets/')&&item.name.endsWith('.js')).map(item=>({name:item.name.split('/').pop(),bytes:item.decodedBodySize}))")
  assert.ok(!firstAssets.some(item => /Markdown|ChatPage|KnowledgePage/.test(item.name)), 'Heavy routes must not load on the dashboard')
  results.push({ initialAssets: firstAssets, initialJSBytes: firstAssets.reduce((sum, item) => sum + item.bytes, 0) })

  await nav('/onboarding')
  await evaluate("(()=>{const node=document.querySelectorAll('.task-card select')[1];node.value='done';node.dispatchEvent(new Event('change',{bubbles:true}))})()")
  await wait("document.body.innerText.includes('Изменения сохранены.')")
  assert.equal((await api('/context')).onboarding.checklist[1].status, 'done')
  await nav('/chat')
  await fill('#chat-message', 'Черновик после обновления Chrome')
  await nav('/')
  await nav('/chat')
  assert.equal(await evaluate("document.querySelector('#chat-message').value"), 'Черновик после обновления Chrome')
  await reload('/chat')
  assert.equal(await evaluate("document.querySelector('#chat-message').value"), 'Черновик после обновления Chrome')
  await fill('#chat-message', 'Какой мой прогресс?')
  await clickExpression("document.querySelector('[aria-label=\"Отправить сообщение\"]')")
  await wait("document.querySelectorAll('.message').length===2 && !document.querySelector('#chat-message').disabled")
  assert.equal(await evaluate("document.querySelector('#chat-message').value"), '')
  assert.ok(await evaluate("document.querySelectorAll('.message-assistant .markdown strong').length>0"))
  const longText = 'Проверка длинного сообщения: ' + 'слово '.repeat(300) + '\n\n```ts\nconst value = "' + 'x'.repeat(240) + '";\n```'
  await fill('#chat-message', longText)
  await clickExpression("document.querySelector('[aria-label=\"Отправить сообщение\"]')")
  await wait("!!document.querySelector('.message-user pre code') && !document.querySelector('#chat-message').disabled")
  assert.ok(await evaluate("document.documentElement.scrollWidth<=innerWidth"))
  await scenario('Оформить отгул')
  await wait("!!document.querySelector('.status-draft_confirmation_required') && !document.querySelector('#chat-message').disabled")
  const requestId = (await api('/context')).corporate_requests[0].id
  await click('Подтвердить и отправить')
  await wait("!!document.querySelector('.status-pending_approval') && !document.querySelector('#chat-message').disabled")
  await nav('/requests'); await role('mentor')
  await fill('.review-form textarea', 'Комментарий наставника после перезагрузки')
  await reload('/requests')
  assert.equal(await evaluate("document.querySelector('.review-form textarea').value"), 'Комментарий наставника после перезагрузки')
  await role('manager')
  assert.equal(await evaluate("document.querySelector('.review-form textarea').value"), '')
  await role('mentor')
  assert.equal(await evaluate("document.querySelector('.review-form textarea').value"), 'Комментарий наставника после перезагрузки')
  await click('Согласовать')
  await wait("!!document.querySelector('.status-approved') && !document.querySelector('.app-shell[aria-busy=\"true\"]')")
  assert.equal((await api('/context')).corporate_requests.find(item => item.id === requestId).status, 'approved')
  await nav('/chat')
  assert.equal((await api('/chat/messages')).find(item => item.action_card?.id === requestId).action_card.status, 'approved')
  await role('intern'); await scenario('Оформить отгул')
  await wait("!!document.querySelector('.status-draft_confirmation_required') && !document.querySelector('#chat-message').disabled")
  await click('Подтвердить и отправить')
  await wait("!!document.querySelector('.status-pending_approval') && !document.querySelector('#chat-message').disabled")
  await nav('/requests'); await role('mentor'); await click('Отклонить')
  await wait("!!document.querySelector('.status-rejected') && !document.querySelector('.app-shell[aria-busy=\"true\"]')")
  await role('intern'); await nav('/chat')
  await scenario('Получить доступ'); assert.ok(await evaluate("!!document.querySelector('.status-approved')"))
  await scenario('Подготовить 1:1'); assert.ok(await evaluate("!!document.querySelector('.status-completed')"))
  if (!production) { await scenario('Проверить права'); assert.ok(await evaluate("!!document.querySelector('.status-blocked_by_rbac')")) }
  else assert.ok(!await evaluate("[...document.querySelectorAll('.scenario-card')].some(node=>node.textContent.includes('Проверить права'))"))
  await clickExpression("document.querySelector('.processing-steps summary')")
  assert.ok(await evaluate("!!document.querySelector('.processing-steps[open]')"))
  await clickExpression("document.querySelector('.citations a')")
  await ready('/knowledge'); await wait("!!document.querySelector('.article-reader .markdown')")
  results.push('Chat, long text/code blocks, Markdown, history, citations, checklist, access, absence confirmation, approval/rejection, comment drafts and ' + (production ? '1:1; development RBAC preset hidden' : '1:1/RBAC'))

  for (const [value, count] of [['intern', 4], ['mentor', 5], ['manager', 6], ['intern', 4]]) {
    await role(value)
    await wait(`document.querySelectorAll('.knowledge-card').length===${count}`)
    assert.equal((await api('/context')).current_user.role, value)
  }
  await fill('#knowledge-query', 'zzzz-no-match-final'); await click('Найти')
  await wait("document.body.innerText.includes('Ничего не найдено')")
  await click('Сбросить поиск'); await wait("document.querySelectorAll('.knowledge-card').length===4")
  await nav('/chat'); await evaluate('history.back()'); await ready('/knowledge')
  await evaluate('history.forward()'); await ready('/chat')
  const loadedBefore = requests.filter(url => /\/assets\/.*\.js$/.test(url))
  await nav('/knowledge'); await nav('/chat')
  assert.equal(requests.filter(url => /\/assets\/.*\.js$/.test(url)).length, loadedBefore.length, 'Already loaded route chunks must not be requested again')
  results.push('Role profiles and knowledge permissions, search/empty state, Back/Forward and cached route chunks')

  expectedErrors = true
  await cdp('Network.setBlockedURLs', { urls: [base + '/api/v1/chat/message'] })
  await fill('#chat-message', 'Вопрос при обрыве сети')
  await clickExpression("document.querySelector('[aria-label=\"Отправить сообщение\"]')")
  await wait("!!document.querySelector('[role=\"alert\"]') && document.body.innerText.includes('Результат операции не подтверждён')")
  assert.equal(await evaluate("document.querySelector('#chat-message').value"), 'Вопрос при обрыве сети')
  await cdp('Network.setBlockedURLs', { urls: [] })
  await click('Обновить данные'); await wait("document.body.innerText.includes('Данные обновлены.')")
  await clickExpression("document.querySelector('[aria-label=\"Отправить сообщение\"]')")
  await wait("document.querySelector('#chat-message').value==='' && !document.querySelector('#chat-message').disabled")
  expectedErrors = false
  results.push('Interrupted API request: visible error, retained draft, explicit read recovery and manual send')

  for (const [width, height] of [[1920,1080],[1440,900],[1366,768],[768,1024],[390,844]]) {
    await cdp('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: width < 700 })
    for (const route of ['/', '/chat', '/onboarding', '/knowledge', '/requests']) {
      await go(route)
      await reload(route)
      const layout = await evaluate(`({width:innerWidth,scrollWidth:document.documentElement.scrollWidth,title:document.querySelector('h1').textContent,missingLabels:[...document.querySelectorAll('input,select,textarea')].filter(node=>!node.labels?.length&&!node.getAttribute('aria-label')).length})`)
      assert.ok(layout.scrollWidth <= width, JSON.stringify({ route, width, height, layout }))
      assert.equal(layout.missingLabels, 0)
      if (width === 1440 || width === 390) {
        layout.accessibility = await accessibility()
        assert.deepEqual(layout.accessibility.contrast, [], 'Text contrast: ' + route)
        if (width === 390) assert.deepEqual(layout.accessibility.smallTargets, [], 'Touch target size: ' + route)
      }
      layouts.push({ route, width, height, ...layout })
      if (width === 390 || width === 1440) await screenshot(`${width}-${route.slice(1) || 'dashboard'}`)
    }
  }
  await clickExpression("document.querySelector('[aria-label=\"Открыть меню\"]')")
  await wait("!!document.querySelector('[role=\"dialog\"]')")
  assert.equal(await evaluate("document.activeElement.getAttribute('aria-label')"), 'Закрыть меню')
  await evaluate("[...document.querySelector('.sidebar').querySelectorAll('a[href],button:not(:disabled)')].at(-1).focus()")
  await cdp('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Tab', code: 'Tab' })
  assert.equal(await evaluate("document.activeElement.getAttribute('aria-label')"), 'AI-buddy — Главная')
  assert.ok(await evaluate("parseFloat(getComputedStyle(document.activeElement).outlineWidth)>=2 && getComputedStyle(document.activeElement).outlineStyle!=='none'"))
  await cdp('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape' })
  await wait("!document.querySelector('[role=\"dialog\"]')")
  assert.equal(await evaluate("document.activeElement.getAttribute('aria-label')"), 'Открыть меню')
  await clickExpression("document.querySelector('[aria-label=\"Открыть меню\"]')")
  await screenshot('mobile-menu')
  await nav('/chat')
  assert.ok(await evaluate("!document.querySelector('[role=\"dialog\"]')"))
  await evaluate("document.querySelector('#chat-message').scrollIntoView({block:'center',behavior:'instant'});document.querySelector('#chat-message').focus()")
  assert.ok(await evaluate("(()=>{const r=document.querySelector('#chat-message').getBoundingClientRect();return r.top>=0&&r.bottom<=innerHeight&&r.right<=innerWidth})()"))
  assert.ok(await evaluate("(()=>{const node=document.createElement('span');node.className='spin';document.body.append(node);const reduced=matchMedia('(prefers-reduced-motion: reduce)').matches&&getComputedStyle(node).animationName==='none';node.remove();return reduced})()"))
  expectedErrors = true
  await cdp('Network.setBlockedURLs', { urls: [base + '/api/v1/chat/message'] })
  await fill('#chat-message', 'Проверка уведомления на мобильном экране')
  await clickExpression("document.querySelector('[aria-label=\"Отправить сообщение\"]')")
  await wait("!!document.querySelector('[role=\"alert\"]')")
  assert.ok(await evaluate("document.documentElement.scrollWidth<=innerWidth"))
  await evaluate("document.querySelector('[role=\"alert\"]').scrollIntoView({block:'center',behavior:'instant'})")
  await screenshot('mobile-network-error')
  await cdp('Network.setBlockedURLs', { urls: [] })
  await click('Обновить данные'); await wait("document.body.innerText.includes('Данные обновлены.')")
  expectedErrors = false
  results.push('25 viewport/route pairs, direct URLs/reloads, keyboard menu focus trap/Escape, labelled forms, reduced motion and reachable composer')

  await cdp('Emulation.setDeviceMetricsOverride', { width: 1440, height: 900, deviceScaleFactor: 1, mobile: false })
  await go('/chat')
  acceptReset = true
  if (!production) {
    await click('Сбросить демо')
    await wait("document.body.innerText.includes('Демо сброшено')")
  } else {
    assert.equal(await evaluate("fetch('/api/v1/chat/reset',{method:'POST'}).then(response=>response.status)"), 200)
    await reload('/chat')
  }
  const reset = await api('/context')
  assert.equal(reset.active_role, 'intern'); assert.equal(reset.onboarding.overall_progress, 0)
  assert.equal(reset.corporate_requests.length, 0)
  assert.deepEqual(await api('/chat/messages'), [])
  assert.deepEqual(errors, [])
  results.push('Reset only in the explicitly isolated memory/fallback server; no unexpected browser exceptions')
  expectedErrors = true
  await cdp('Network.setCacheDisabled', { cacheDisabled: true })
  await go('/')
  await cdp('Network.setBlockedURLs', { urls: [base + '/assets/ChatPage-*.js'] })
  await clickExpression("document.querySelector('.sidebar a[href=\"/chat\"]')")
  await wait("document.body.innerText.includes('Не удалось отобразить страницу')")
  assert.ok(await evaluate("!!document.querySelector('nav[aria-label=\"Основная навигация\"]')"))
  await cdp('Network.setBlockedURLs', { urls: [] })
  const reloadLink = await evaluate("[...document.querySelectorAll('a')].find(node=>node.textContent==='Перезагрузить страницу')?.getAttribute('href')")
  if (!reloadLink) throw new Error('A failed lazy import is cached by React.lazy; a normal boundary retry cannot reload the route module. Provide explicit page reload recovery.')
  await clickExpression("[...document.querySelectorAll('a')].find(node=>node.textContent==='Перезагрузить страницу')")
  await ready('/chat')
  assert.ok(!await evaluate("document.body.innerText.includes('Не удалось отобразить страницу')"))
  results.push('Blocked lazy route chunk: scoped Error Boundary and explicit reload recovery')
  await fs.writeFile(path.join(reportDir, 'report.json'), JSON.stringify({ base, health, results, layouts, errors }, null, 2))
  console.log(JSON.stringify({ results, viewportChecks: layouts.length, reportDir }, null, 2))
} finally {
  await cdp('Network.setBlockedURLs', { urls: [] }).catch(() => {})
  await cdp('Network.setCacheDisabled', { cacheDisabled: false }).catch(() => {})
  socket.close()
}
