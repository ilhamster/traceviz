import { AppCore } from '../app_core/app_core.js';
import { ConfigurationError } from '../errors/errors.js';
import { DataFetcherInterface } from './data_fetcher_interface.js';
import { toObject } from '../protocol/json_request.js';
import { fromObject } from '../protocol/json_response.js';
import type { Request } from '../protocol/request_interface.js';
import type { Response } from '../protocol/response_interface.js';
import {
  catchError,
  defer,
  map,
  mergeMap,
  throwError,
  type Observable,
} from 'rxjs';
import { str } from '../core.js';

/**
 * HttpDataFetcher is a DataFetcher that fetches data via HTTP GET requests.
 * It is framework-agnostic and suitable for any browser-based TraceViz app.
 */
export class HttpDataFetcher implements DataFetcherInterface {
  constructor(
    private readonly core: AppCore,
    private readonly endpoint: string = '/GetData',
  ) {}

  fetch(req: Request): Observable<Response> {
    const reqstr = JSON.stringify(toObject(req));
    const url = new URL(this.endpoint, window.location.origin);
    url.searchParams.set('req', reqstr);
    return defer(() => fetch(url.toString())).pipe(
      mergeMap(async (resp) => {
        const body = await resp.text();
        if (!resp.ok) {
          // Prefer backend-provided error details when available.
          const detail = body.trim();
          throw new Error(`HTTP ${resp.status}: ${detail || resp.statusText}`);
        }
        return body;
      }),
      map((body) => fromObject(body) as Response),
      catchError((err: unknown) => {
        this.core.err(new ConfigurationError(String(err)));
        return throwError(() => err);
      }),
    );
  }
}
