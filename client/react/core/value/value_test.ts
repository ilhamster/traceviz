import 'jasmine';

import {AppCore, StringValue} from '@traceviz/client-core';

import {getGlobalValue} from './global.ts';

describe('react/core/value', () => {
  it('getGlobalValue returns the named global value', () => {
    const core = new AppCore();
    const val = new StringValue('alpha');
    core.globalState.set('collection_name', val);

    const got = getGlobalValue(core, 'collection_name');
    expect(got).toBe(val);
    val.val = 'beta';
    expect((got as StringValue).val).toBe('beta');
  });
});
