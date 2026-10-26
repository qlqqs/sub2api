"""Local-only workbench fixtures; no remote credentials, no changes to finance fixtures."""
import os
import sys
import json
sys.path.insert(0, '/home/qlqq/workspace/sub2api-local/e2e')
os.environ['E2E_RUN_DIR'] = '/tmp/s2a-workbench-e2e'
from lib import STATE, HTTP, GATEWAY, admin, admin_token, as_user, register, uid, q, q1, x, save_state, user_token

if STATE.get('workbench_ready'):
    print('existing workbench fixtures ready')
    raise SystemExit(0)
group = admin('POST', '/groups', {'name': 'Codex 稳定分组', 'platform': 'openai', 'subscription_type': 'standard',
    'is_exclusive': False, 'rate_multiplier': 1, 'allow_image_generation': True})
account = admin('POST', '/accounts', {'name': 'e2e-workbench-' + STATE['run'], 'platform': 'openai', 'type': 'apikey',
    'concurrency': 20, 'credentials': {'api_key': 'sk-poc-chat-upstream', 'base_url': 'http://sub2api-local-upstream-mock:18998',
    'model_mapping': {'gpt-6.1-sol': 'gpt-6.1-sol', 'gpt-image-2.5-sunburst': 'gpt-image-2.5-sunburst'}}, 'group_ids': [group['id']]})
original = q1("SELECT value FROM settings WHERE key='signup_workbench_group_id'")
try:
    x(f"INSERT INTO settings(key,value,updated_at) VALUES('signup_workbench_group_id','{group['id']}',NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()")
    for tag in ('WB', 'WZERO', 'WNONE'):
        register(tag)
        keys = as_user(tag, 'GET', '/keys')['items']
        assert len(keys) == 1, (tag, len(keys))
        assert keys[0]['name'] == group['name'] and keys[0]['group_id'] == group['id']
        state = as_user(tag, 'GET', '/user/workbench/studio')
        assert state['eligible'] and state['reasoning_allowed'] and not state['initialized'], state
        assert state['text_model'] == 'gpt-6.1-sol' and state['reasoning_effort'] == 'medium'
        STATE['users'][tag]['key_id'] = keys[0]['id']
        x(f"UPDATE users SET balance={100 if tag == 'WB' else 0} WHERE id={uid(tag)}")
        if tag == 'WNONE':
            as_user(tag, 'DELETE', f"/keys/{keys[0]['id']}")
        if tag == 'WZERO':
            r = HTTP.get(GATEWAY + '/v1/models', headers={'Authorization': 'Bearer ' + keys[0]['key']}, timeout=30)
            assert r.status_code == 200, (r.status_code, r.text[:250])
            ids = [m['id'] for m in r.json()['data']]
            assert 'gpt-6.1-sol' in ids and 'gpt-image-2.5-sunburst' in ids, ids
            r = HTTP.post(GATEWAY + '/v1/images/generations', headers={'Authorization': 'Bearer ' + keys[0]['key']},
                json={'model': 'gpt-image-2.5-sunburst', 'prompt': 'zero balance test', 'n': 1}, timeout=30)
            assert r.status_code == 403 and 'INSUFFICIENT_BALANCE' in r.text, (r.status_code, r.text[:200])
    # A broken target must not prevent registration.
    x("UPDATE settings SET value='9000000000' WHERE key='signup_workbench_group_id'")
    register('WFAIL')
    assert as_user('WFAIL', 'GET', '/keys')['items'] == []
    # Existing administrator does not become eligible merely by opening a workbench.
    old = HTTP.get(GATEWAY + '/api/v1/user/workbench/studio', headers={'Authorization':'Bearer '+admin_token()},timeout=30).json()['data']
    assert not old['eligible']
finally:
    if original:
        value = original['value'].replace("'", "''")
        x(f"UPDATE settings SET value='{value}',updated_at=NOW() WHERE key='signup_workbench_group_id'")
    else:
        x("DELETE FROM settings WHERE key='signup_workbench_group_id'")
STATE['workbench_ready'] = True
STATE['workbench_group_id'] = group['id']
STATE['workbench_account_id'] = account['id']
save_state()
print(json.dumps({'group_id': group['id'], 'users': {tag: {'id': u['id'], 'email': u['email']} for tag, u in STATE['users'].items()}}, ensure_ascii=False))
print('PASS: signup/default name/group/eligibility/model discovery/zero balance/fail-open/existing user; target setting restored')
