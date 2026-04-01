import { useState } from 'react'
import { CommandCenter } from './pages/CommandCenter'
import { OrderMatrix } from './pages/OrderMatrix'
import { OrderDetail } from './pages/OrderDetail'
import { PeerNodes } from './pages/PeerNodes'
import { JITChannels } from './pages/JITChannels'
import { Analytics } from './pages/Analytics'
import { ResponsiveLayout } from './components/ResponsiveLayout'
import { Login } from './pages/Login'
import { Settings } from './pages/Settings'
import { useAuth } from './hooks/useAuth'
import { EventsProvider } from './contexts/EventsContext'

import { motion, AnimatePresence } from 'framer-motion'

function App() {
  const { user, loading } = useAuth()
  const [currentPage, setCurrentPage] = useState('overview')
  const [selectedOrderId, setSelectedOrderId] = useState<string | null>(null)

  if (loading) return null

  if (!user) {
    return <Login />
  }

  const handleViewOrder = (orderId: string) => {
    setSelectedOrderId(orderId);
    setCurrentPage('order-detail');
  }

  const handleBackFromOrder = () => {
    setSelectedOrderId(null);
    setCurrentPage('orders');
  }

  const renderPage = () => {
    switch (currentPage) {
      case 'overview': return <CommandCenter onViewOrder={handleViewOrder} />;
      case 'orders': return <OrderMatrix onViewOrder={handleViewOrder} />;
      case 'order-detail': return selectedOrderId
        ? <OrderDetail orderId={selectedOrderId} onBack={handleBackFromOrder} />
        : <OrderMatrix onViewOrder={handleViewOrder} />;
      case 'nodes': return <PeerNodes />;
      case 'jit': return <JITChannels />;
      case 'analytics': return <Analytics />;
      case 'settings': return <Settings />;
      default: return <CommandCenter onViewOrder={handleViewOrder} />;
    }
  }

  return (
    <EventsProvider>
      <ResponsiveLayout activePage={currentPage} onPageChange={setCurrentPage}>
        <AnimatePresence mode="wait">
          <motion.div
             key={currentPage + (selectedOrderId || '')}
             initial={{ opacity: 0, y: 10 }}
             animate={{ opacity: 1, y: 0 }}
             exit={{ opacity: 0, y: -10 }}
             transition={{ duration: 0.3, ease: "easeOut" }}
          >
            {renderPage()}
          </motion.div>
        </AnimatePresence>
      </ResponsiveLayout>
    </EventsProvider>
  )
}

export default App
