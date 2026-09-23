import { Component, computed, effect, inject, input, signal } from '@angular/core';
import { ButtonModule } from '@openng/optimus-ui/button';
import { CardModule } from '@openng/optimus-ui/card';
import { ChartModule } from '@openng/optimus-ui/chart';
import { MessageModule } from '@openng/optimus-ui/message';
import { ProgressSpinnerModule } from '@openng/optimus-ui/progressspinner';
import { SelectModule } from '@openng/optimus-ui/select';
import { TagModule } from '@openng/optimus-ui/tag';
import { FormsModule } from '@angular/forms';
import { DatePipe } from '@angular/common';

import { Device, DevicesService, Measurement, MeasurementsService } from '../../api';
import {
  commandsFor,
  formatValue,
  isControllable,
  kindIcon,
  kindLabel,
  metricLabel,
  metricUnit,
  parseState,
} from '../../core/device-state';
import { DevicesStore } from '../../core/devices.store';

@Component({
  selector: 'app-device-detail',
  imports: [
    DatePipe,
    FormsModule,
    CardModule,
    ButtonModule,
    TagModule,
    ChartModule,
    SelectModule,
    MessageModule,
    ProgressSpinnerModule,
  ],
  templateUrl: './device-detail.html',
  styleUrl: './device-detail.css',
})
export class DeviceDetail {
  /** Identifiant issu de la route, injecté par withComponentInputBinding(). */
  readonly id = input.required<string>();

  private readonly devicesApi = inject(DevicesService);
  private readonly measurementsApi = inject(MeasurementsService);
  protected readonly store = inject(DevicesStore);

  protected readonly kindLabel = kindLabel;
  protected readonly kindIcon = kindIcon;
  protected readonly isControllable = isControllable;
  protected readonly commandsFor = commandsFor;

  protected readonly device = signal<Device | null>(null);
  protected readonly error = signal<string | null>(null);
  protected readonly measurements = signal<Measurement[]>([]);
  protected readonly selectedMetric = signal<string | null>(null);

  /** États courants, sous forme de couples libellé/valeur. */
  protected readonly readings = computed(() => {
    const device = this.device();
    if (!device) {
      return [];
    }
    return Object.entries(parseState(device))
      .map(([metric, value]) => ({
        metric,
        label: metricLabel(metric),
        value: formatValue(metric, value),
      }))
      .sort((a, b) => a.label.localeCompare(b.label, 'fr'));
  });

  /** Grandeurs pour lesquelles un historique existe. */
  protected readonly metricOptions = computed(() => {
    const metrics = new Set(this.measurements().map((m) => m.metric));
    return [...metrics]
      .sort((a, b) => a.localeCompare(b, 'fr'))
      .map((metric) => ({ label: metricLabel(metric), value: metric }));
  });

  /** Données du graphique pour la grandeur sélectionnée. */
  protected readonly chartData = computed(() => {
    const metric = this.selectedMetric();
    if (!metric) {
      return null;
    }

    // Le backend renvoie du plus récent au plus ancien ; un graphique se lit
    // dans l'autre sens.
    const points = this.measurements()
      .filter((m) => m.metric === metric)
      .slice()
      .reverse();

    if (points.length === 0) {
      return null;
    }

    const unit = metricUnit(metric);
    return {
      labels: points.map((p) => new Date(p.recorded_at).toLocaleString('fr-FR', {
        day: '2-digit',
        month: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })),
      datasets: [
        {
          label: unit ? `${metricLabel(metric)} (${unit})` : metricLabel(metric),
          data: points.map((p) => p.value),
          borderColor: '#10b981',
          backgroundColor: 'rgba(16, 185, 129, 0.12)',
          fill: true,
          tension: 0.3,
          pointRadius: points.length > 60 ? 0 : 2,
        },
      ],
    };
  });

  protected readonly chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: { legend: { display: true, position: 'top' as const } },
    scales: {
      x: { ticks: { maxTicksLimit: 10, autoSkip: true } },
      y: { beginAtZero: false },
    },
  };

  constructor() {
    // input() étant un signal, cet effect se relance quand la route change
    // d'identifiant sans réinstancier le composant.
    effect(() => {
      const id = this.id();
      this.load(id);
      this.loadMeasurements(id);
    });
  }

  private load(id: string): void {
    this.devicesApi.getDevice(id).subscribe({
      next: (device) => {
        this.device.set(device);
        this.error.set(null);
      },
      error: () => {
        this.device.set(null);
        this.error.set("Cet équipement est introuvable.");
      },
    });
  }

  private loadMeasurements(id: string): void {
    // Une semaine d'historique : assez pour lire une tendance, assez peu pour
    // que le graphique reste lisible et la réponse légère.
    const from = new Date(Date.now() - 7 * 24 * 3600 * 1000).toISOString();

    this.measurementsApi.listMeasurements(id, undefined, from, undefined, 2000).subscribe({
      next: (response) => {
        this.measurements.set(response.measurements);

        // Présélectionner la première grandeur évite un graphique vide à
        // l'ouverture.
        const metrics = this.metricOptions();
        if (metrics.length > 0 && !this.selectedMetric()) {
          this.selectedMetric.set(metrics[0].value);
        }
      },
      error: () => this.measurements.set([]),
    });
  }

  protected send(command: string): void {
    const device = this.device();
    if (device) {
      this.store.sendCommand(device, command);
    }
  }
}
