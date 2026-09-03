import { Link } from 'react-router-dom'
import { useDocumentPage } from '../hooks/useDocumentPage'
import SiteHeader from '../components/SiteHeader'

export function AuthFeature({ title, children }) {
  return <div className="feature"><i>✓</i><div><b>{title}</b><p>{children}</p></div></div>
}

export default function AuthLayout({ title, login = false, children, aside }) {
  useDocumentPage({ title, bodyClass: 'register-page' })

  return (
    <>
      <SiteHeader />
      <main className={`register-main${login ? ' login-main' : ''}`}>
        {children}
        {aside}
      </main>
    </>
  )
}

export function AuthSwitchLink({ prompt, to, children }) {
  return <p className="login-prompt">{prompt} <Link to={to}>{children}</Link></p>
}
