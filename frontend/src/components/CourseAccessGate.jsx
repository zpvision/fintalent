import { Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import PublicLayout from '../layouts/PublicLayout'

export default function CourseAccessGate({ children }) {
  const { coursesAccess, coursesAccessLoading } = useAuth()
  if (coursesAccessLoading) return <PublicLayout><main className="course-page"><p role="status">Загрузка…</p></main></PublicLayout>
  if (!coursesAccess) return <PublicLayout><main className="course-page"><h1>Раздел недоступен</h1><p>Сейчас доступ к этому разделу ограничен.</p><Link to="/">На главную</Link></main></PublicLayout>
  return children
}
