import SiteHeader from '../components/SiteHeader'
import { useContext } from 'react'
import { AcademyShellContext } from './AcademyShellContext'

export default function PublicLayout({ children }) {
  const academyShell = useContext(AcademyShellContext)
  if (academyShell) return children
  return (
    <>
      <SiteHeader />
      {children}
    </>
  )
}
