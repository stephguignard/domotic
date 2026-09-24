import { HttpTestingController } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { RoomEditor } from './room-editor';
import { Device } from '../../api';
import { makeRelay, testProviders } from '../../testing/providers';

describe('RoomEditor', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [RoomEditor],
      providers: testProviders(),
    }).compileComponents();

    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  async function render(device: Device): Promise<ComponentFixture<RoomEditor>> {
    const fixture = TestBed.createComponent(RoomEditor);
    fixture.componentRef.setInput('device', device);
    await fixture.whenStable();
    return fixture;
  }

  function el(fixture: ComponentFixture<RoomEditor>): HTMLElement {
    return fixture.nativeElement as HTMLElement;
  }

  async function openEditor(fixture: ComponentFixture<RoomEditor>): Promise<HTMLInputElement> {
    el(fixture).querySelector<HTMLButtonElement>('.room-display button')!.click();
    await fixture.whenStable();
    return el(fixture).querySelector<HTMLInputElement>('input[name="room"]')!;
  }

  const url = (d: Device) => `/api/devices/${encodeURIComponent(d.id)}/room`;

  it('affiche « Sans pièce » pour un équipement non rangé', async () => {
    const fixture = await render(makeRelay());
    expect(el(fixture).textContent).toContain('Sans pièce');
  });

  it('enregistre la pièce saisie, espaces retirés', async () => {
    const relay = makeRelay();
    const fixture = await render(relay);

    const input = await openEditor(fixture);
    input.value = '  Buanderie ';
    el(fixture).querySelector('form')!.dispatchEvent(new Event('submit'));
    await fixture.whenStable();

    const req = http.expectOne(url(relay));
    expect(req.request.method).toBe('PUT');
    expect(req.request.body).toEqual({ room: 'Buanderie' });
    req.flush({ ...relay, room: 'Buanderie', room_overridden: true });
    // Revenu en lecture.
    expect(el(fixture).querySelector('form')).toBeNull();
  });

  it('propose de revenir à la pièce de la source quand elle a été changée', async () => {
    const relay = makeRelay({ room: 'Buanderie', source_room: '', room_overridden: true });
    const fixture = await render(relay);

    await openEditor(fixture);
    const reset = el(fixture).querySelector<HTMLElement>('.reset-room button')!;
    expect(reset.textContent).toContain('sans pièce');
    reset.click();

    const req = http.expectOne(url(relay));
    expect(req.request.method).toBe('DELETE');
    req.flush({ ...relay, room: '', room_overridden: false });
  });

  it('ne propose pas de retour à la source quand la pièce en vient déjà', async () => {
    const fixture = await render(makeRelay());
    await openEditor(fixture);
    expect(el(fixture).querySelector('.reset-room')).toBeNull();
  });

  it('abandonne la saisie avec Échap, sans rien envoyer', async () => {
    const fixture = await render(makeRelay());
    const input = await openEditor(fixture);

    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await fixture.whenStable();

    expect(el(fixture).querySelector('form')).toBeNull();
  });
});
