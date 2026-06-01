import { createTheme } from '@mui/material/styles'
import { blue, cyan, teal } from '@mui/material/colors'

const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: {
      main: cyan[700],
    },
    secondary: {
      main: teal[500],
    },
    background: {
      default: '#0a1929',
      paper: '#0d2137',
    },
  },
  typography: {
    fontFamily: '"Roboto", "Noto Sans SC", "Microsoft YaHei", sans-serif',
  },
  shape: {
    borderRadius: 12,
  },
  components: {
    MuiCard: {
      styleOverrides: {
        root: {
          backgroundImage: 'none',
          backgroundColor: 'rgba(13, 33, 55, 0.8)',
          backdropFilter: 'blur(10px)',
          border: '1px solid rgba(255, 255, 255, 0.08)',
        },
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: {
          backgroundImage: 'none',
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: {
          textTransform: 'none',
          borderRadius: 8,
        },
      },
    },
  },
})

export default theme
