(() => {
  if (window.__fintalentAdminRouting) return
  window.__fintalentAdminRouting = true

  const navRoutes = {
    'dictionary-nav': '/admin/dictionaries',
    'users-nav': '/admin/users',
    'publications-nav': '/admin/publications',
    'survey-nav': '/admin/applicant-survey',
    'vacancy-survey-nav': '/admin/vacancy-survey',
    'admin-testing-nav': '/admin/testing',
    'other-dictionaries-nav': '/admin/other-dictionaries/categories',
    'admin-duties-nav': '/admin/duties/categories',
    'client-exchange-nav': '/admin/client-exchange/employee_range',
    'accounting-company-nav': '/admin/accounting-companies/directions',
    'community-nav': '/admin/community',
    'help-topics-nav': '/admin/help-topics',
    'profimarket-admin-nav': '/admin/profimarket/purchases',
  }

  let restoring = false

  function setPath(path, replace = false) {
    if (`${location.pathname}${location.search}` === path) return
    history[replace ? 'replaceState' : 'pushState']({}, '', path)
  }

  function waitFor(selector, timeout = 15000) {
    const found = document.querySelector(selector)
    if (found) return Promise.resolve(found)
    return new Promise((resolve, reject) => {
      const observer = new MutationObserver(() => {
        const element = document.querySelector(selector)
        if (!element) return
        observer.disconnect()
        clearTimeout(timer)
        resolve(element)
      })
      const timer = setTimeout(() => {
        observer.disconnect()
        reject(new Error(`Admin route target not found: ${selector}`))
      }, timeout)
      observer.observe(document.documentElement, { childList: true, subtree: true, attributes: true, attributeFilter: ['class'] })
    })
  }

  async function clickWhenReady(selector) {
    const element = await waitFor(selector)
    element.click()
    return element
  }

  function waitForNavHandler(id, timeout = 15000) {
    return new Promise((resolve, reject) => {
      const started = Date.now()
      const check = () => {
        const element = document.getElementById(id)
        if (element && typeof element.onclick === 'function') {
          resolve(element)
          return
        }
        if (Date.now() - started >= timeout) {
          reject(new Error(`Admin navigation handler not ready: ${id}`))
          return
        }
        setTimeout(check, 50)
      }
      check()
    })
  }

  async function restoreRoute() {
    restoring = true
    try {
      await waitFor('html[data-admin-ready="true"]', 3600000)
      const parts = location.pathname.split('/').filter(Boolean).slice(1)
      const section = parts[0] || 'dictionaries'
      const detail = parts[1]
      const leaf = parts[2]
      const navBySection = {
        dictionaries: 'dictionary-nav', users: 'users-nav', publications: 'publications-nav',
        'applicant-survey': 'survey-nav', 'vacancy-survey': 'vacancy-survey-nav', testing: 'admin-testing-nav',
        'other-dictionaries': 'other-dictionaries-nav', duties: 'admin-duties-nav',
        'client-exchange': 'client-exchange-nav', 'accounting-companies': 'accounting-company-nav', community: 'community-nav',
        'help-topics': 'help-topics-nav', profimarket: 'profimarket-admin-nav',
      }
      const navID = navBySection[section] || 'dictionary-nav'
      const navButton = await waitForNavHandler(navID)
      navButton.click()

      if (section === 'dictionaries' && /^\d+$/.test(detail || '')) {
        await clickWhenReady(`.dictionary-card[data-id="${detail}"]`)
      } else if (section === 'testing' && /^\d+$/.test(detail || '')) {
        await clickWhenReady(`#test-table tbody tr[data-id="${detail}"]`)
        if (leaf === 'attempts') {
          await clickWhenReady('#admin-testing [data-tab="attempts"]')
          if (/^\d+$/.test(parts[3] || '')) await clickWhenReady(`#admin-testing .attempt[data-id="${parts[3]}"]`)
        }
      } else if (section === 'other-dictionaries' && detail) {
        await clickWhenReady(`#other-dictionaries-section [data-other-tab="${CSS.escape(detail)}"]`)
      } else if (section === 'duties' && detail) {
        await clickWhenReady(`#admin-duties-section [data-duty-tab="${CSS.escape(detail)}"]`)
        const category = new URLSearchParams(location.search).get('category')
        if (detail === 'duties' && category) {
          const select = await waitFor('#admin-duties-section #duties-category')
          select.value = category
          select.dispatchEvent(new Event('change'))
        }
      } else if (section === 'client-exchange' && detail) {
        await clickWhenReady(`#client-exchange-admin [data-kind="${CSS.escape(detail)}"]`)
      } else if (section === 'accounting-companies' && detail) {
        await clickWhenReady(`#accounting-company-admin [data-kind="${CSS.escape(detail)}"]`)
      } else if (section === 'profimarket' && detail) {
        await clickWhenReady(`#profimarket-admin [data-pm-tab="${CSS.escape(detail)}"]`)
      }
    } catch (error) {
      console.warn(error.message)
    } finally {
      restoring = false
    }
  }

  document.addEventListener('click', (event) => {
    if (restoring) return
    const target = event.target.closest('button, .dictionary-card, #test-table tbody tr, .attempt')
    if (!target) return
    const navButton = target.closest('.sidebar nav button')
    if (navButton && navRoutes[navButton.id]) {
      setPath(navRoutes[navButton.id])
      return
    }
    if (target.matches('.dictionary-card[data-id]')) setPath(`/admin/dictionaries/${target.dataset.id}`)
    else if (target.matches('#test-table tbody tr[data-id]')) setPath(`/admin/testing/${target.dataset.id}`)
    else if (target.id === 'tb') setPath('/admin/testing')
    else if (target.id === 'attempt-back') setPath(location.pathname.split('/').slice(0, 4).join('/'))
    else if (target.matches('.attempt[data-id]')) setPath(`${location.pathname.replace(/\/attempts(?:\/.*)?$/, '')}/attempts/${target.dataset.id}`)
    else if (target.dataset.tab === 'attempts' && target.closest('#admin-testing')) setPath(`${location.pathname.replace(/\/attempts(?:\/.*)?$/, '')}/attempts`)
    else if (target.dataset.tab === 'questions' && target.closest('#admin-testing')) setPath(location.pathname.replace(/\/attempts(?:\/.*)?$/, ''))
    else if (target.dataset.otherTab) setPath(`/admin/other-dictionaries/${target.dataset.otherTab}`)
    else if (target.dataset.dutyTab) setPath(`/admin/duties/${target.dataset.dutyTab}`)
    else if (target.dataset.categoryVariants) setPath(`/admin/duties/duties?category=${target.dataset.categoryVariants}`)
    else if (target.dataset.pmTab) setPath(`/admin/profimarket/${target.dataset.pmTab}`)
    else if (target.dataset.kind && target.closest('#client-exchange-admin')) setPath(`/admin/client-exchange/${target.dataset.kind}`)
    else if (target.dataset.kind && target.closest('#accounting-company-admin')) setPath(`/admin/accounting-companies/${target.dataset.kind}`)
    else if (target.id === 'back') setPath('/admin/dictionaries')
  }, true)

  window.addEventListener('popstate', restoreRoute)
  if (location.pathname === '/admin' || location.pathname === '/admin/') setPath('/admin/dictionaries', true)
  restoreRoute()
})()
