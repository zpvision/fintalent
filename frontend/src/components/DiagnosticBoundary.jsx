import { Component } from 'react'

export default class DiagnosticBoundary extends Component {
  state = { failed: false }
  static getDerivedStateFromError() { return { failed: true } }
  componentDidCatch(error) {
    window.reportFinTalentError?.({ code: 'react_error', stack: error?.stack })
  }
  render() {
    if (this.state.failed) return <main role="alert" style={{ maxWidth: 640, margin: '48px auto', padding: 24 }}><h1>Не удалось открыть страницу</h1><p>Попробуйте обновить страницу. Если ошибка повторится, вернитесь позже.</p><button type="button" onClick={() => window.location.reload()}>Обновить страницу</button> <a href="/">На главную</a></main>
    return this.props.children
  }
}
