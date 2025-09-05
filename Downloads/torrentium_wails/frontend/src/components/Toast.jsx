import React from 'react'
import { X, CheckCircle, AlertCircle, Info, AlertTriangle } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { Button } from './ui/Button'
import { cn } from '../lib/utils'

const toastIcons = {
  success: CheckCircle,
  error: AlertCircle,
  warning: AlertTriangle,
  info: Info
}

const toastStyles = {
  success: 'border-success bg-success/10 text-success',
  error: 'border-error bg-error/10 text-error',
  warning: 'border-warning bg-warning/10 text-warning',
  info: 'border-primary bg-primary/10 text-primary'
}

export function Toast({ notification }) {
  const { dispatch } = useApp()
  const Icon = toastIcons[notification.type] || Info

  const handleClose = () => {
    dispatch({ type: 'REMOVE_NOTIFICATION', payload: notification.id })
  }

  return (
    <div className={cn(
      "flex items-center space-x-3 p-4 rounded-lg border shadow-lg min-w-[300px] toast-enter",
      toastStyles[notification.type]
    )}>
      <Icon className="h-5 w-5 flex-shrink-0" />
      <div className="flex-1">
        <p className="text-sm font-medium">{notification.message}</p>
      </div>
      <Button
        variant="ghost"
        size="icon"
        onClick={handleClose}
        className="h-6 w-6 text-current hover:bg-current/20"
      >
        <X className="h-4 w-4" />
      </Button>
    </div>
  )
}

export function ToastContainer() {
  const { notifications } = useApp()

  if (notifications.length === 0) return null

  return (
    <div className="fixed top-4 right-4 z-50 space-y-2">
      {notifications.map((notification) => (
        <Toast key={notification.id} notification={notification} />
      ))}
    </div>
  )
}
