import SiteHeader from '../components/SiteHeader'

export default function PublicLayout({ children }) {
  return (
    <>
      <SiteHeader />
      {children}
    </>
  )
}
