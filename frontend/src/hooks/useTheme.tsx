import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { useAuth } from './useAuth'

type Theme = 'twilight' | 'glacium'

interface ThemeContextType {
    theme: Theme
    toggleTheme: () => void
}

const ThemeContext = createContext<ThemeContextType | null>(null)

export function ThemeProvider({ children }: { children: ReactNode }) {
    const { user } = useAuth()

    const [theme, setTheme] = useState<Theme>(() => {
        const saved = localStorage.getItem('theme') as Theme
        return (saved === 'twilight' || saved === 'glacium') ? saved : 'twilight'
    })

    // Enforce theme based on role
    useEffect(() => {
        if (user) {
            if (user.role === 'super_admin' || user.role === 'admin') {
                setTheme('twilight')
            } else {
                setTheme('glacium')
            }
        } else {
            // Default to twilight for login screen/logged out state
            setTheme('twilight')
        }
    }, [user])

    useEffect(() => {
        document.documentElement.setAttribute('data-theme', theme)
        localStorage.setItem('theme', theme)
    }, [theme])

    const toggleTheme = () => {
        // Toggle disabled/soft-disabled as theme is role-enforced, 
        // but leaving function for dev testing if needed
        setTheme(prev => prev === 'twilight' ? 'glacium' : 'twilight')
    }

    return (
        <ThemeContext.Provider value={{ theme, toggleTheme }}>
            {children}
        </ThemeContext.Provider>
    )
}

export function useTheme() {
    const context = useContext(ThemeContext)
    if (!context) {
        throw new Error('useTheme must be used within ThemeProvider')
    }
    return context
}
