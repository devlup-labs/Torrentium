import React from 'react'
import { cn } from '../../lib/utils'

const Badge = React.forwardRef(({ className, variant, ...props }, ref) => {
  const variants = {
    default: "bg-primary text-background",
    secondary: "bg-secondary text-white",
    destructive: "bg-error text-white",
    outline: "border border-primary text-primary",
    success: "bg-success text-white",
    warning: "bg-warning text-white"
  }

  return (
    <div
      ref={ref}
      className={cn(
        "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2",
        variants[variant || "default"],
        className
      )}
      {...props}
    />
  )
})
Badge.displayName = "Badge"

export { Badge }
