import { ApplicationConfig, provideBrowserGlobalErrorListeners } from '@angular/core';
import { provideRouter, withComponentInputBinding } from '@angular/router';
import { provideHttpClient, withFetch } from '@angular/common/http';
import { MessageService } from '@openng/optimus-ui/api';
import { provideOptimus } from '@openng/optimus-ui/config';
import Aura from '@openng/optimus-ui-themes/aura';

import { routes } from './app.routes';
import { provideApi } from './api';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideRouter(routes, withComponentInputBinding()),
    provideHttpClient(withFetch()),
    provideOptimus({ theme: { preset: Aura } }),

    // MessageService alimente le <p-toast> du composant racine. Fourni ici
    // plutôt que dans chaque composant, pour que tous partagent la même file.
    MessageService,

    // Le client généré cible une base vide : en développement, le proxy de
    // l'Angular CLI relaie /api vers le backend ; en production, le backend
    // sert lui-même le frontend, l'origine est donc la bonne.
    provideApi(''),
  ],
};
