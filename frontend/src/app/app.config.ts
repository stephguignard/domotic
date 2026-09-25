import { registerLocaleData } from '@angular/common';
import localeFr from '@angular/common/locales/fr';
import { ApplicationConfig, LOCALE_ID, provideBrowserGlobalErrorListeners } from '@angular/core';
import { provideRouter, withComponentInputBinding } from '@angular/router';
import { provideHttpClient, withFetch } from '@angular/common/http';
import { ConfirmationService, MessageService } from '@openng/optimus-ui/api';
import { provideOptimus } from '@openng/optimus-ui/config';
import Aura from '@openng/optimus-ui-themes/aura';

import { routes } from './app.routes';
import { provideApi } from './api';

// Noms des jours et des mois en français pour DatePipe (« ven. 25 sept. »).
registerLocaleData(localeFr);

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    { provide: LOCALE_ID, useValue: 'fr-FR' },
    provideRouter(routes, withComponentInputBinding()),
    provideHttpClient(withFetch()),
    // cssLayer place les styles des composants Optimus dans une couche nommée,
    // insérée entre `base` et `components` par styles.css. Sans cela, le
    // preflight de Tailwind passerait après eux et écraserait leur mise en
    // forme ; avec, les classes utilitaires restent prioritaires sur les
    // composants, ce qui permet de les ajuster ponctuellement.
    provideOptimus({
      theme: {
        preset: Aura,
        options: {
          cssLayer: { name: 'optimus', order: 'theme, base, optimus, components, utilities' },
        },
      },
    }),

    // MessageService alimente le <p-toast> du composant racine. Fourni ici
    // plutôt que dans chaque composant, pour que tous partagent la même file.
    MessageService,
    // Même logique pour le <p-confirmdialog> qui confirme les commandes sensibles.
    ConfirmationService,

    // Le client généré cible une base vide : en développement, le proxy de
    // l'Angular CLI relaie /api vers le backend ; en production, le backend
    // sert lui-même le frontend, l'origine est donc la bonne.
    provideApi(''),
  ],
};
