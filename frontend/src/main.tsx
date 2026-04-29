import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { AuthProvider } from './contexts/AuthContext'
import { UnitProvider } from './contexts/UnitContext'
import { RouterProvider } from '@tanstack/react-router'
import { router } from './router'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AuthProvider>
      <UnitProvider>
        <RouterProvider router={router} />
      </UnitProvider>
    </AuthProvider>
  </StrictMode>,
)
