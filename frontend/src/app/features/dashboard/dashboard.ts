import { Component, computed, effect, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ButtonModule } from '@openng/optimus-ui/button';
import { CardModule } from '@openng/optimus-ui/card';
import { MessageModule } from '@openng/optimus-ui/message';
import { ProgressSpinnerModule } from '@openng/optimus-ui/progressspinner';
import { SelectModule } from '@openng/optimus-ui/select';
import { TagModule } from '@openng/optimus-ui/tag';

import { Device } from '../../api';
import {
  NO_ROOM,
  commandsFor,
  formatValue,
  isControllable,
  kindIcon,
  kindLabel,
  metricLabel,
  litColor,
  litGlow,
  parseState,
  powerState,
  sourceLabel,
  sourceSeverity,
} from '../../core/device-state';
import { DevicesStore } from '../../core/devices.store';
import { ScenesStore } from '../../core/scenes.store';
import { RoomOrder } from './room-order';

/** Une grandeur prête à afficher sur une carte. */
interface Reading {
  metric: string;
  label: string;
  value: string;
}

/** Clé des filtres mémorisés dans le navigateur. */
const FILTERS_KEY = 'domotic.dashboard.filters';

interface Filters {
  kind: string | null;
  room: string | null;
}

/**
 * Relit les filtres de la dernière visite. Le stockage peut être indisponible
 * (navigation privée, données effacées) : on repart alors sans filtre.
 */
function loadFilters(): Filters {
  try {
    const raw = localStorage.getItem(FILTERS_KEY);
    const parsed = raw ? (JSON.parse(raw) as Partial<Filters>) : {};
    return {
      kind: typeof parsed.kind === 'string' ? parsed.kind : null,
      room: typeof parsed.room === 'string' ? parsed.room : null,
    };
  } catch {
    return { kind: null, room: null };
  }
}

@Component({
  selector: 'app-dashboard',
  imports: [
    RouterLink,
    FormsModule,
    CardModule,
    ButtonModule,
    TagModule,
    MessageModule,
    ProgressSpinnerModule,
    SelectModule,
    RoomOrder,
  ],
  templateUrl: './dashboard.html',
})
export class Dashboard {
  protected readonly store = inject(DevicesStore);
  protected readonly scenes = inject(ScenesStore);

  /** Filtres rapides, combinés : un type ET une pièce. */
  protected readonly kindFilter = signal<string | null>(null);
  protected readonly roomFilter = signal<string | null>(null);

  constructor() {
    this.scenes.load();

    const saved = loadFilters();
    this.kindFilter.set(saved.kind);
    this.roomFilter.set(saved.room);
    effect(() => {
      const filters: Filters = { kind: this.kindFilter(), room: this.roomFilter() };
      try {
        localStorage.setItem(FILTERS_KEY, JSON.stringify(filters));
      } catch {
        // Stockage indisponible : les filtres ne survivront pas au rechargement.
      }
    });
  }

  /** Types représentés, par libellé. */
  protected readonly kindOptions = computed(() =>
    [...new Set(this.store.devices().map((d) => d.kind))]
      .map((kind) => ({ label: kindLabel(kind), value: kind }))
      .sort((a, b) => a.label.localeCompare(b.label, 'fr')),
  );

  /** Pièces représentées, dans l'ordre d'affichage, « Sans pièce » en dernier. */
  protected readonly roomOptions = computed(() => {
    const options = this.store.rooms().map((room) => ({ label: room, value: room }));
    if (this.store.devices().some((d) => !d.room)) {
      options.push({ label: NO_ROOM, value: NO_ROOM });
    }
    return options;
  });

  protected readonly filtered = computed(() => this.kindFilter() !== null || this.roomFilter() !== null);

  /** Groupes par pièce, restreints par les filtres ; les pièces vidées disparaissent. */
  protected readonly groups = computed(() => {
    const kind = this.kindFilter();
    const room = this.roomFilter();
    return this.store
      .byRoom()
      .filter((g) => room === null || g.room === room)
      .map((g) => ({ ...g, devices: g.devices.filter((d) => kind === null || d.kind === kind) }))
      .filter((g) => g.devices.length > 0);
  });

  protected readonly visibleCount = computed(() =>
    this.groups().reduce((n, g) => n + g.devices.length, 0),
  );

  protected resetFilters(): void {
    this.kindFilter.set(null);
    this.roomFilter.set(null);
  }

  protected readonly kindLabel = kindLabel;
  protected readonly kindIcon = kindIcon;
  protected readonly isControllable = isControllable;
  protected readonly commandsFor = commandsFor;
  protected readonly sourceLabel = sourceLabel;
  protected readonly powerState = powerState;
  protected readonly litColor = litColor;
  protected readonly litGlow = litGlow;
  protected readonly sourceSeverity = sourceSeverity;

  /**
   * Grandeurs à afficher sur la carte d'un équipement.
   *
   * L'état complet peut contenir une vingtaine d'entrées — la plupart sans
   * intérêt d'un coup d'œil. En montrer quatre garde les cartes lisibles ; le
   * détail reste accessible d'un clic.
   */
  protected readings(device: Device): Reading[] {
    return Object.entries(parseState(device))
      .filter(([, value]) => value !== null && value !== undefined)
      .slice(0, 4)
      .map(([metric, value]) => ({
        metric,
        label: metricLabel(metric),
        value: formatValue(metric, value),
      }));
  }

  protected send(event: Event, device: Device, command: string): void {
    // La carte entière est un lien vers le détail : sans cela, agir sur un
    // volet ferait aussi naviguer.
    event.stopPropagation();
    event.preventDefault();
    this.store.sendCommand(device, command);
  }
}
