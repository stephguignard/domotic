/** Extrait un message lisible d'une erreur HTTP. */
export function describeError(err: unknown): string {
  if (typeof err === 'object' && err !== null) {
    // Le backend renvoie du RFC 7807 : le champ `detail` porte le message utile.
    const body = (err as { error?: { detail?: string; title?: string } }).error;
    if (body?.detail) {
      return body.detail;
    }
    if (body?.title) {
      return body.title;
    }
    const message = (err as { message?: string }).message;
    if (message) {
      return message;
    }
  }
  return 'Erreur inattendue';
}
