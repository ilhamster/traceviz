import '@mantine/core/styles.css';
import '@mantine/notifications/styles.css';

import React from 'react';
import ReactDOM from 'react-dom/client';
import { MantineProvider } from '@mantine/core';
import { Notifications } from '@mantine/notifications';

import App from './App.tsx';
import './index.css';

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <MantineProvider>
      <Notifications position="bottom-right" />
      <App />
    </MantineProvider>
  </React.StrictMode>,
);
