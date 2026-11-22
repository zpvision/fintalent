import React from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { AuthProvider } from './context/AuthContext'
import './styles/react.css'
import DiagnosticBoundary from './components/DiagnosticBoundary'

createRoot(document.getElementById('root')).render(
  <DiagnosticBoundary><BrowserRouter>
    <AuthProvider>
      <App />
    </AuthProvider>
  </BrowserRouter></DiagnosticBoundary>,
)
