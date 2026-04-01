import { 
  createRouter, 
  createRootRoute, 
  createRoute, 
  Outlet
} from '@tanstack/react-router'
import { z } from 'zod'
import { CommandCenter } from './pages/CommandCenter'
import { OrderMatrix } from './pages/OrderMatrix'
import { OrderDetail } from './pages/OrderDetail'
import { PeerNodes } from './pages/PeerNodes'
import { JITChannels } from './pages/JITChannels'
import { Analytics } from './pages/Analytics'
import { Settings } from './pages/Settings'
import { Login } from './pages/Login'
import { ResponsiveLayout } from './components/ResponsiveLayout'
import { useAuth } from './hooks/useAuth'
import { EventsProvider } from './contexts/EventsContext'
import { motion, AnimatePresence } from 'framer-motion'

// Root Route
const rootRoute = createRootRoute({
  component: RootComponent,
})

function RootComponent() {
  const { user, loading } = useAuth()
  
  if (loading) return null

  if (!user) {
    return <Login />
  }

  return (
    <EventsProvider>
      <RootLayout />
    </EventsProvider>
  )
}

function RootLayout() {
  // Helper to map pathname to legacy page names
  const getActivePage = (path: string) => {
    if (path === '/') return 'overview'
    if (path.startsWith('/orders')) return 'orders'
    if (path === '/nodes') return 'nodes'
    if (path === '/jit') return 'jit'
    if (path === '/analytics') return 'analytics'
    if (path === '/settings') return 'settings'
    return 'overview'
  }

  return (
    <ResponsiveLayout 
      activePage={getActivePage(window.location.pathname)} 
    >
      <AnimatePresence mode="wait">
        <motion.div
           key={window.location.pathname}
           initial={{ opacity: 0, y: 10 }}
           animate={{ opacity: 1, y: 0 }}
           exit={{ opacity: 0, y: -10 }}
           transition={{ duration: 0.3, ease: "easeOut" }}
        >
          <Outlet />
        </motion.div>
      </AnimatePresence>
    </ResponsiveLayout>
  )
}

// Routes
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: () => <CommandCenter onViewOrder={(id) => router.navigate({ to: `/orders/${id}` })} />,
})

// Search param schema for orders
const orderSearchSchema = z.object({
  q: z.string().optional().catch(''),
})

const ordersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/orders',
  validateSearch: (search) => orderSearchSchema.parse(search),
  component: OrdersPage,
})

function OrdersPage() {
  const { q } = ordersRoute.useSearch()
  return (
    <OrderMatrix 
      onViewOrder={(id) => router.navigate({ to: `/orders/${id}` })} 
      initialSearch={q || ''} 
    />
  )
}

const orderDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/orders/$orderId',
  component: OrderDetailPage,
})

function OrderDetailPage() {
  const { orderId } = orderDetailRoute.useParams()
  return (
    <OrderDetail 
      orderId={orderId} 
      onBack={() => router.navigate({ to: '/orders' })} 
    />
  )
}

const nodesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/nodes',
  component: NodesPage,
})

function NodesPage() {
  return (
    <PeerNodes 
      onViewPeerOrders={(peerId) => {
        router.navigate({ to: '/orders', search: { q: peerId } })
      }} 
    />
  )
}

const jitRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/jit',
  component: JITChannels,
})

const analyticsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/analytics',
  component: Analytics,
})

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/settings',
  component: Settings,
})

// Create the route tree
const routeTree = rootRoute.addChildren([
  indexRoute,
  ordersRoute,
  orderDetailRoute,
  nodesRoute,
  jitRoute,
  analyticsRoute,
  settingsRoute,
])

// Create the router
export const router = createRouter({ routeTree })

// Register for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
