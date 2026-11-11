import { useEffect, useState } from 'react'
import { Link, Outlet, useParams } from 'react-router-dom'
import PublicLayout from '../../layouts/PublicLayout'
import { AcademyShellContext } from '../../layouts/AcademyShellContext'
import usePageStyles, { PageStylesContext } from '../../hooks/usePageStyles'
import { styles } from '../courses/shared'
import { cohortStyles } from './CohortShared'
import { communicationStyles } from './CommunicationShared'
import { academyAPI } from '../../api/academy'
import AcademyCourseHeader from './AcademyCourseHeader'

const shellStyles = [...new Set([...styles, ...cohortStyles, ...communicationStyles,
 '/static/course-editor-theme.css?v=1', '/static/academy.css?v=4',
 '/static/academy-analytics.css?v=2', '/static/academy-acquisition.css?v=1'])]

// Remains mounted across course tabs; author forms retain their own save guards.
export default function AcademyLayout() {
 const { id } = useParams(), [workspace,setWorkspace] = useState(null), [error,setError] = useState('')
 usePageStyles(shellStyles)
 useEffect(()=>{let alive=true;setWorkspace(null);setError('');academyAPI.workspaceSummary(id).then(w=>{if(alive)setWorkspace({id,course:w.course})}).catch(e=>{if(alive)setError(e.message)});return()=>{alive=false}},[id])
 const course=workspace?.id===id?workspace.course:null
 return <PublicLayout><div className="course-page academy-page academy-shell">
  <nav className="course-breadcrumbs" aria-label="Хлебные крошки"><Link to="/profile/courses">Мои курсы</Link><span>›</span><span>{course?.title || 'Академия'}</span></nav>
  {course ? <AcademyCourseHeader course={course} id={id} /> : <p role={error?'alert':'status'}>{error || 'Открываем кабинет курса…'}</p>}
  {course && <AcademyShellContext.Provider value={true}><PageStylesContext.Provider value={shellStyles}><div className="academy-shell-body"><Outlet /></div></PageStylesContext.Provider></AcademyShellContext.Provider>}
 </div></PublicLayout>
}
