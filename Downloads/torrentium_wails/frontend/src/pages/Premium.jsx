import React from 'react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { 
  Crown, 
  Zap, 
  Shield, 
  Download, 
  Upload, 
  Users, 
  Star,
  Check,
  Sparkles,
  Rocket,
  Globe,
  Lock,
  BarChart3,
  Clock,
  Archive
} from 'lucide-react';

export default function Premium() {
  const features = [
    {
      icon: Zap,
      title: "Unlimited Speed",
      description: "No bandwidth restrictions or speed limits",
      premium: true
    },
    {
      icon: Download,
      title: "Priority Downloads",
      description: "Get priority access to seeds and faster downloads",
      premium: true
    },
    {
      icon: Upload,
      title: "Enhanced Seeding",
      description: "Advanced seeding algorithms for better ratios",
      premium: true
    },
    {
      icon: Shield,
      title: "VPN Integration",
      description: "Built-in VPN protection for anonymous torrenting",
      premium: true
    },
    {
      icon: Users,
      title: "Premium Swarms",
      description: "Access to premium-only swarms with verified peers",
      premium: true
    },
    {
      icon: BarChart3,
      title: "Advanced Analytics",
      description: "Detailed statistics and performance insights",
      premium: true
    },
    {
      icon: Archive,
      title: "Cloud Backup",
      description: "Backup your torrent collection to the cloud",
      premium: true
    },
    {
      icon: Globe,
      title: "Global CDN",
      description: "Access content through our global CDN network",
      premium: true
    }
  ];

  const plans = [
    {
      name: "Free",
      price: "$0",
      period: "forever",
      description: "Perfect for casual users",
      features: [
        "Basic P2P sharing",
        "Standard download speeds",
        "Community support",
        "Basic encryption",
        "5 concurrent downloads"
      ],
      current: true
    },
    {
      name: "Premium",
      price: "$9.99",
      period: "month",
      description: "For power users who want more",
      features: [
        "Unlimited speed & bandwidth",
        "Priority downloads",
        "VPN integration",
        "Premium swarms access",
        "Advanced analytics",
        "Cloud backup (100GB)",
        "Priority support",
        "Unlimited downloads"
      ],
      popular: true
    },
    {
      name: "Pro",
      price: "$19.99",
      period: "month",
      description: "Maximum performance and features",
      features: [
        "Everything in Premium",
        "Global CDN access",
        "Enhanced seeding rewards",
        "Custom tracker support",
        "API access",
        "Cloud backup (1TB)",
        "24/7 premium support",
        "Early feature access"
      ]
    }
  ];

  const stats = [
    { label: "Active Premium Users", value: "24,891", icon: Users },
    { label: "Average Speed Boost", value: "340%", icon: Zap },
    { label: "Premium Content", value: "1.2M+", icon: Archive },
    { label: "Uptime Guarantee", value: "99.9%", icon: Shield }
  ];

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="text-center">
        <div className="flex items-center justify-center gap-3 mb-4">
          <Crown className="h-8 w-8 text-warning" />
          <h1 className="text-3xl font-bold text-text">Torrentium Premium</h1>
        </div>
        <p className="text-lg text-text-secondary max-w-2xl mx-auto">
          Unlock the full potential of P2P sharing with premium features designed for power users
        </p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {stats.map((stat, index) => (
          <Card key={index} className="p-4 text-center">
            <stat.icon className="h-6 w-6 text-primary mx-auto mb-2" />
            <div className="text-2xl font-bold text-text">{stat.value}</div>
            <div className="text-sm text-text-secondary">{stat.label}</div>
          </Card>
        ))}
      </div>

      {/* Pricing Plans */}
      <div>
        <h2 className="text-2xl font-bold text-text text-center mb-8">Choose Your Plan</h2>
        <div className="grid md:grid-cols-3 gap-6">
          {plans.map((plan, index) => (
            <Card key={index} className={`p-6 relative ${plan.popular ? 'ring-2 ring-primary' : ''}`}>
              {plan.popular && (
                <div className="absolute -top-3 left-1/2 transform -translate-x-1/2">
                  <Badge className="bg-primary text-background px-3 py-1">
                    <Star className="h-3 w-3 mr-1" />
                    Most Popular
                  </Badge>
                </div>
              )}
              
              <div className="text-center mb-6">
                <h3 className="text-xl font-bold text-text mb-2">{plan.name}</h3>
                <div className="mb-2">
                  <span className="text-3xl font-bold text-text">{plan.price}</span>
                  <span className="text-text-secondary">/{plan.period}</span>
                </div>
                <p className="text-sm text-text-secondary">{plan.description}</p>
              </div>
              
              <ul className="space-y-3 mb-6">
                {plan.features.map((feature, featureIndex) => (
                  <li key={featureIndex} className="flex items-center gap-2">
                    <Check className="h-4 w-4 text-success flex-shrink-0" />
                    <span className="text-sm text-text-secondary">{feature}</span>
                  </li>
                ))}
              </ul>
              
              <Button 
                className="w-full" 
                variant={plan.current ? "outline" : plan.popular ? "default" : "secondary"}
                disabled={plan.current}
              >
                {plan.current ? "Current Plan" : `Upgrade to ${plan.name}`}
              </Button>
            </Card>
          ))}
        </div>
      </div>

      {/* Features Grid */}
      <div>
        <h2 className="text-2xl font-bold text-text text-center mb-8">Premium Features</h2>
        <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-4">
          {features.map((feature, index) => (
            <Card key={index} className="p-4 text-center">
              <div className="relative inline-block mb-3">
                <feature.icon className="h-8 w-8 text-primary" />
                {feature.premium && (
                  <Crown className="h-4 w-4 text-warning absolute -top-1 -right-1" />
                )}
              </div>
              <h3 className="font-semibold text-text mb-2">{feature.title}</h3>
              <p className="text-sm text-text-secondary">{feature.description}</p>
            </Card>
          ))}
        </div>
      </div>

      {/* Testimonials */}
      <Card className="p-8">
        <h2 className="text-2xl font-bold text-text text-center mb-8">What Premium Users Say</h2>
        <div className="grid md:grid-cols-3 gap-6">
          <div className="text-center">
            <div className="mb-4">
              <div className="flex justify-center mb-2">
                {[...Array(5)].map((_, i) => (
                  <Star key={i} className="h-4 w-4 text-warning fill-current" />
                ))}
              </div>
              <p className="text-sm text-text-secondary italic">
                "The speed boost is incredible! Downloads that used to take hours now finish in minutes."
              </p>
            </div>
            <div className="text-sm font-medium text-text">- Alex M., Pro User</div>
          </div>
          
          <div className="text-center">
            <div className="mb-4">
              <div className="flex justify-center mb-2">
                {[...Array(5)].map((_, i) => (
                  <Star key={i} className="h-4 w-4 text-warning fill-current" />
                ))}
              </div>
              <p className="text-sm text-text-secondary italic">
                "Premium swarms are a game changer. Always connected to quality peers with great speeds."
              </p>
            </div>
            <div className="text-sm font-medium text-text">- Sarah K., Premium User</div>
          </div>
          
          <div className="text-center">
            <div className="mb-4">
              <div className="flex justify-center mb-2">
                {[...Array(5)].map((_, i) => (
                  <Star key={i} className="h-4 w-4 text-warning fill-current" />
                ))}
              </div>
              <p className="text-sm text-text-secondary italic">
                "The VPN integration gives me peace of mind. Anonymous and secure torrenting made easy."
              </p>
            </div>
            <div className="text-sm font-medium text-text">- Mike R., Pro User</div>
          </div>
        </div>
      </Card>

      {/* FAQ */}
      <Card className="p-6">
        <h2 className="text-xl font-bold text-text mb-6">Frequently Asked Questions</h2>
        <div className="space-y-4">
          <div>
            <h3 className="font-semibold text-text mb-2">Can I cancel my subscription anytime?</h3>
            <p className="text-sm text-text-secondary">Yes, you can cancel your subscription at any time. You'll continue to have access to premium features until the end of your billing period.</p>
          </div>
          
          <div>
            <h3 className="font-semibold text-text mb-2">Is there a money-back guarantee?</h3>
            <p className="text-sm text-text-secondary">We offer a 30-day money-back guarantee for all premium subscriptions. If you're not satisfied, contact us for a full refund.</p>
          </div>
          
          <div>
            <h3 className="font-semibold text-text mb-2">What payment methods do you accept?</h3>
            <p className="text-sm text-text-secondary">We accept all major credit cards, PayPal, and cryptocurrency payments for maximum privacy and convenience.</p>
          </div>
          
          <div>
            <h3 className="font-semibold text-text mb-2">Can I use premium on multiple devices?</h3>
            <p className="text-sm text-text-secondary">Yes, your premium subscription can be used on up to 5 devices simultaneously. Perfect for users with multiple computers.</p>
          </div>
        </div>
      </Card>

      {/* CTA */}
      <Card className="p-8 text-center bg-gradient-to-r from-primary/10 to-secondary/10 border-primary/20">
        <Sparkles className="h-12 w-12 text-primary mx-auto mb-4" />
        <h2 className="text-2xl font-bold text-text mb-4">Ready to Go Premium?</h2>
        <p className="text-text-secondary mb-6 max-w-md mx-auto">
          Join thousands of satisfied users who have unlocked the full potential of P2P sharing
        </p>
        <div className="flex justify-center gap-4">
          <Button size="lg" className="px-8">
            <Rocket className="h-4 w-4 mr-2" />
            Start Premium Trial
          </Button>
          <Button variant="outline" size="lg">
            Compare Plans
          </Button>
        </div>
      </Card>
    </div>
  );
}
