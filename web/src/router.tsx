import {
  createRouter,
  createRootRoute,
  createRoute,
  createHashHistory,
  Outlet,
  useLocation
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
  const { user } = useAuth()
  
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
  const location = useLocation()

  return (
    <ResponsiveLayout>
      <AnimatePresence mode="wait">
        <motion.div
           key={location.pathname}
           initial={{ opacity: 0 }}
           animate={{ opacity: 1 }}
           exit={{ opacity: 0 }}
           transition={{ duration: 0.2, ease: "linear" }}
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

// Hash history keeps the path in the fragment (#/orders/123) so the server
// always receives "/" — no server-side catch-all needed.
const hashHistory = createHashHistory()
export const router = createRouter({ routeTree, history: hashHistory })

// Register for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
