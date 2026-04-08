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
            background: '#111111',
            color: '#f5f5f5',
            border: '1px solid #2a2a2a',
          },
        }}
      />
    </BrowserRouter>
  )
}

export default App
