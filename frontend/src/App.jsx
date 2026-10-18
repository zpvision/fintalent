import { useEffect } from 'react'
import { matchPath, Navigate, Route, Routes, useLocation, useNavigate, useParams } from 'react-router-dom'
import LoginPage from './pages/auth/LoginPage'
import ForgotPasswordPage from './pages/auth/ForgotPasswordPage'
import RegisterPage from './pages/auth/RegisterPage'
import CatalogPage from './pages/catalog/CatalogPage'
import HomePage from './pages/home/HomePage'
import MarketplacePage from './pages/marketplace/MarketplacePage'
import AccountingCompaniesPage from './pages/companies/AccountingCompaniesPage'
import AccountingCompanyViewPage from './pages/companies/AccountingCompanyViewPage'
import ProfiMarketPage from './pages/profimarket/ProfiMarketPage'
import PublicationsPage from './pages/publications/PublicationsPage'
import VacancyViewPage from './pages/vacancies/VacancyViewPage'
import ResumeViewPage from './pages/resumes/ResumeViewPage'
import TestsPage from './pages/tests/TestsPage'
import ClientExchangePage from './pages/clientExchange/ClientExchangePage'
import ClientExchangeCreatePage from './pages/clientExchange/ClientExchangeCreatePage'
import ClientExchangeAccess from './pages/clientExchange/ClientExchangeAccess'
import AccountingCompanyPassportPage from './pages/companies/AccountingCompanyPassportPage'
import AccountingCompanyCreatePage from './pages/companies/AccountingCompanyCreatePage'
import TestTakePage from './pages/tests/TestTakePage'
import EmployeeTestPage from './pages/tests/EmployeeTestPage'
import EmployeeResultPage from './pages/tests/EmployeeResultPage'
import TestCreatePage from './pages/tests/TestCreatePage'
import VacancyCreatePage from './pages/vacancies/VacancyCreatePage'
import PublicationAnalyticsPage from './pages/publications/PublicationAnalyticsPage'
import ProfiMarketMyPage from './pages/profimarket/ProfiMarketMyPage'
import ProfiMarketDetailPage from './pages/profimarket/ProfiMarketDetailPage'
import ProfiMarketCreatePage from './pages/profimarket/ProfiMarketCreatePage'
import ProfiMarketRegulationEditPage from './pages/profimarket/ProfiMarketRegulationEditPage'
import ProfiMarketProductEditPage from './pages/profimarket/ProfiMarketProductEditPage'
import ResumeCreatePage from './pages/resumes/ResumeCreatePage'
import MarketplaceCreateTestPage from './pages/marketplace/MarketplaceCreateTestPage'
import ProfilePage from './pages/profile/ProfilePage'
import ProfilePurposePage from './pages/profile/ProfilePurposePage'
import PublicationEditorPage from './pages/publications/PublicationEditorPage'
import AdminPage from './pages/admin/AdminPage'
import PublicLayout from './layouts/PublicLayout'
import CourseAccessGate from './components/CourseAccessGate'
import CoursesPage from './pages/courses/CoursesPage'
import CourseEditor from './pages/courses/CourseEditor'
import CourseView from './pages/courses/CourseView'
import AcademyPage from './pages/academy/AcademyPage'
import AcademyStudents from './pages/academy/AcademyStudents'
import AcademyReviews from './pages/academy/AcademyReviews'
import MyLearningPage from './pages/academy/MyLearningPage'
import LearningRoom from './pages/academy/LearningRoom'

const reactPaths = [
 '/profile/courses/:id/academy/reviews',
 '/profile/learning', '/profile/learning/:enrollmentId', '/profile/courses/:id/academy/students', '/profile/courses/:id/academy/room/:versionId',
  '/profile/courses/:id/academy', '/courses', '/courses/:slug', '/profile/courses', '/profile/courses/create', '/profile/courses/:id/edit', '/profile/courses/:id/preview',
  '/', '/login', '/register', '/forgot-password', '/profile-purpose', '/vacancies', '/vacancies/view', '/vacancies/create',
  '/profiles', '/profiles/view/:id', '/profiles/create', '/resumes', '/resume/view/:id', '/resume/create', '/marketplace', '/marketplace/create-test',
  '/accounting-companies', '/accounting-companies/view', '/accounting-companies/passport', '/accounting-companies/create',
  '/profimarket', '/profimarket/my', '/profimarket/solution/:key', '/profimarket/create', '/profimarket/regulation/edit', '/profimarket/product/edit',
  '/publications', '/publications/saved', '/publications/create', '/publications/analytics', '/publications/:id/edit',
  '/tests', '/tests/take', '/employee-test', '/employee-result', '/client-exchange', '/client-exchange/create', '/profile', '/admin/*',
]

const isolatedPaths = [
  '/profimarket/regulation/edit',
  '/profiles/create',
  '/publications/create',
  '/publications/:id/edit',
  '/admin/*',
]

function matchesAnyPath(pathname, paths) {
  return paths.some((path) => matchPath({ path, end: true }, pathname))
}

function requiresDocumentNavigation(from, to) {
  return matchesAnyPath(from, isolatedPaths) || matchesAnyPath(to, isolatedPaths)
}

function ReactNavigationBridge() {
  const navigate = useNavigate()
  const location = useLocation()
  useEffect(() => {
    function handleNavigation(event) {
      event.preventDefault()
      const url = new URL(event.detail.to, window.location.href)
      if (requiresDocumentNavigation(location.pathname, url.pathname)) {
        if (event.detail.replace) window.location.replace(url.href)
        else window.location.assign(url.href)
        return
      }
      navigate(event.detail.to, { replace: event.detail.replace })
    }
    function handleHardNavigation(event) {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
      const anchor = event.target.closest?.('a[href]')
      if (!anchor || anchor.target || anchor.hasAttribute('download')) return
      const href = anchor.getAttribute('href')
      if (!href || href === '#' || href.startsWith('#')) return
      const url = new URL(anchor.href, window.location.href)
      if (url.origin !== window.location.origin || !matchesAnyPath(url.pathname, reactPaths)) return
      if (!requiresDocumentNavigation(location.pathname, url.pathname)) return
      event.preventDefault()
      event.stopPropagation()
      window.location.assign(`${url.pathname}${url.search}${url.hash}`)
    }
    function handleClick(event) {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
      const anchor = event.target.closest?.('a[href]')
      if (!anchor || anchor.target || anchor.hasAttribute('download')) return
      const href = anchor.getAttribute('href')
      if (!href || href === '#' || href.startsWith('#')) return
      const url = new URL(anchor.href, window.location.href)
      if (url.origin !== window.location.origin || !reactPaths.some((path) => matchPath({ path, end: true }, url.pathname))) return
      event.preventDefault()
      navigate(`${url.pathname}${url.search}${url.hash}`)
    }
    document.addEventListener('click', handleHardNavigation, true)
    document.addEventListener('click', handleClick)
    window.addEventListener('fintalent:navigate', handleNavigation)
    return () => {
      document.removeEventListener('click', handleHardNavigation, true)
      document.removeEventListener('click', handleClick)
      window.removeEventListener('fintalent:navigate', handleNavigation)
    }
  }, [location.pathname, navigate])
  return null
}

function LegacyRoute() {
  const location = useLocation()
  useEffect(() => {
    if (import.meta.env.DEV) window.location.replace(`http://127.0.0.1:8080${location.pathname}${location.search}${location.hash}`)
  }, [location])
  if (import.meta.env.DEV) return null
  return <Navigate to="/vacancies" replace />
}

function ProfileRoute() {
  return <ProfilePage />
}

function LegacyProfilesRoute({ page }) {
  const location = useLocation()
  const { id } = useParams()
  const target = page === 'view' ? `/profiles/view/${encodeURIComponent(id)}` : page === 'create' ? '/profiles/create' : '/profiles'
  return <Navigate to={`${target}${location.search}${location.hash}`} replace />
}

function ProfiMarketMyRoute() {
  const location = useLocation()
  const tab = new URLSearchParams(location.search).get('tab')
  if (tab === 'favorites') return <ProfiMarketMyPage />
  if (tab === 'purchases') return <Navigate to="/profile?section=profimarket-purchases" replace />
  if (tab === 'orders') return <Navigate to="/profile?section=profimarket&tab=orders" replace />
  return <Navigate to="/profile?section=profimarket" replace />
}

export default function App() {
  return (
    <><ReactNavigationBridge /><Routes>
      <Route path="/courses" element={<CourseAccessGate><CoursesPage /></CourseAccessGate>} />
      <Route path="/courses/:slug" element={<CourseAccessGate><CourseView /></CourseAccessGate>} />
      <Route path="/profile/courses" element={<CourseAccessGate><CoursesPage mine /></CourseAccessGate>} />
      <Route path="/profile/courses/create" element={<CourseAccessGate><CourseEditor /></CourseAccessGate>} />
      <Route path="/profile/learning" element={<MyLearningPage />} />
      <Route path="/profile/learning/:enrollmentId" element={<LearningRoom />} />
      <Route path="/profile/courses/:id/academy/reviews" element={<CourseAccessGate><AcademyReviews /></CourseAccessGate>} />
      <Route path="/profile/courses/:id/academy/students" element={<CourseAccessGate><AcademyStudents /></CourseAccessGate>} />
      <Route path="/profile/courses/:id/academy/room/:versionId" element={<CourseAccessGate><LearningRoom preview /></CourseAccessGate>} />
      <Route path="/profile/courses/:id/academy" element={<CourseAccessGate><AcademyPage /></CourseAccessGate>} />
      <Route path="/profile/courses/:id/edit" element={<CourseAccessGate><CourseEditor /></CourseAccessGate>} />
      <Route path="/profile/courses/:id/preview" element={<CourseAccessGate><CourseView preview /></CourseAccessGate>} />
      <Route path="/admin/courses/:id/preview" element={<CourseView admin />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/forgot-password" element={<ForgotPasswordPage />} />
      <Route path="/profile-purpose" element={<PublicLayout><ProfilePurposePage /></PublicLayout>} />
      <Route path="/vacancies" element={<CatalogPage type="vacancies" />} />
      <Route path="/vacancies/view" element={<VacancyViewPage />} />
      <Route path="/profiles" element={<CatalogPage type="resumes" />} />
      <Route path="/profiles/view/:id" element={<ResumeViewPage />} />
      <Route path="/resumes" element={<LegacyProfilesRoute />} />
      <Route path="/resume/view/:id" element={<LegacyProfilesRoute page="view" />} />
      <Route path="/marketplace" element={<MarketplacePage />} />
      <Route path="/accounting-companies" element={<AccountingCompaniesPage />} />
      <Route path="/accounting-companies/view" element={<AccountingCompanyViewPage />} />
      <Route path="/profimarket" element={<ProfiMarketPage />} />
      <Route path="/publications" element={<PublicationsPage />} />
      <Route path="/publications/saved" element={<PublicationsPage saved />} />
      <Route path="/tests" element={<TestsPage />} />
      <Route path="/client-exchange" element={<ClientExchangeAccess><ClientExchangePage /></ClientExchangeAccess>} />
      <Route path="/client-exchange/create" element={<ClientExchangeAccess><ClientExchangeCreatePage /></ClientExchangeAccess>} />
      <Route path="/accounting-companies/passport" element={<AccountingCompanyPassportPage />} />
      <Route path="/accounting-companies/create" element={<AccountingCompanyCreatePage />} />
      <Route path="/tests/take" element={<TestTakePage />} />
      <Route path="/employee-test" element={<EmployeeTestPage />} />
      <Route path="/employee-result" element={<EmployeeResultPage />} />
      <Route path="/tests/create" element={<TestCreatePage />} />
      <Route path="/vacancies/create" element={<PublicLayout><VacancyCreatePage /></PublicLayout>} />
      <Route path="/publications/analytics" element={<PublicationAnalyticsPage />} />
      <Route path="/profimarket/my" element={<ProfiMarketMyRoute />} />
      <Route path="/profimarket/solution/:key" element={<ProfiMarketDetailPage />} />
      <Route path="/profimarket/create" element={<ProfiMarketCreatePage />} />
      <Route path="/profimarket/regulation/edit" element={<ProfiMarketRegulationEditPage />} />
      <Route path="/profimarket/product/edit" element={<ProfiMarketProductEditPage />} />
      <Route path="/profiles/create" element={<PublicLayout><ResumeCreatePage /></PublicLayout>} />
      <Route path="/resume/create" element={<LegacyProfilesRoute page="create" />} />
      <Route path="/marketplace/create-test" element={<MarketplaceCreateTestPage />} />
      <Route path="/profile" element={<ProfileRoute />} />
      <Route path="/publications/create" element={<PublicationEditorPage />} />
      <Route path="/publications/:id/edit" element={<PublicationEditorPage />} />
      <Route path="/admin/*" element={<AdminPage />} />
      <Route path="/" element={<HomePage />} />
      <Route path="*" element={<LegacyRoute />} />
    </Routes></>
  )
}
