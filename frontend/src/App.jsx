import { useEffect } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import LoginPage from './pages/auth/LoginPage'
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

function LegacyRoute() {
  const location = useLocation()
  useEffect(() => {
    if (import.meta.env.DEV) window.location.replace(`http://127.0.0.1:8080${location.pathname}${location.search}${location.hash}`)
  }, [location])
  if (import.meta.env.DEV) return null
  return <Navigate to="/vacancies" replace />
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/vacancies" element={<CatalogPage type="vacancies" />} />
      <Route path="/vacancies/view" element={<VacancyViewPage />} />
      <Route path="/resumes" element={<CatalogPage type="resumes" />} />
      <Route path="/resume/view/:id" element={<ResumeViewPage />} />
      <Route path="/marketplace" element={<MarketplacePage />} />
      <Route path="/accounting-companies" element={<AccountingCompaniesPage />} />
      <Route path="/accounting-companies/view" element={<AccountingCompanyViewPage />} />
      <Route path="/profimarket" element={<ProfiMarketPage />} />
      <Route path="/publications" element={<PublicationsPage />} />
      <Route path="/publications/saved" element={<PublicationsPage saved />} />
      <Route path="/" element={<HomePage />} />
      <Route path="*" element={<LegacyRoute />} />
    </Routes>
  )
}
