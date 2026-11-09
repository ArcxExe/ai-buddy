/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        avito: {
          green: "#00AA6C",
          "green-hover": "#008f5a",
          blue: "#00AAFF",
          "blue-hover": "#0090d9",
          red: "#FF4053",
          purple: "#9356FE",
          yellow: "#FFD110",
          dark: "#1E2229",
          gray: "#F2F4F7",
          border: "#E4E7EC",
          muted: "#667085",
        },
      },
    },
  },
  plugins: [],
}
