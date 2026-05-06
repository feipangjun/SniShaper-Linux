/// <reference types="vite/client" />

// Wails runtime type declarations
declare global {
  interface Window {
    runtime?: {
      EventsOn: (eventName: string, callback: (...args: any[]) => void) => () => void;
      EventsOff: (eventName: string) => void;
      EventsEmit: (eventName: string, ...args: any[]) => void;
    };
  }
}

export {};
