import { Toaster } from 'sonner'
import { BrowserRouter } from 'react-router-dom'
import { AppRouter } from '@/app/router'

function App() {
  return (
    <BrowserRouter>
      <AppRouter />
      <Toaster
        position="top-right"
        toastOptions={{
          style: {
            background: '#ffffff',
            color: '#27272a',
            border: '1px solid #e4e4e7',
            boxShadow: '0 16px 40px rgba(31, 36, 48, 0.12)',
          },
        }}
      />
    </BrowserRouter>
  )
}

export default App
