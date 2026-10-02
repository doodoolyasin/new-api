import requests
import json
import time
import concurrent.futures

BASE_URL = "http://127.0.0.1:8085"
ADMIN_KEY = "adm_secret_key_linker_2026_x"

def test_full_suite():
    print("=== STARTING GATEWAY INTEGRATION TEST SUITE ===")
    
    # 1. Test Healthz
    r = requests.get(f"{BASE_URL}/healthz")
    assert r.status_code == 200, f"Healthz failed: {r.text}"
    print("[PASS] 1. Healthz endpoint OK")

    # 2. Test Models
    r = requests.get(f"{BASE_URL}/v1/models")
    assert r.status_code == 200, f"Models failed: {r.text}"
    print("[PASS] 2. Models list endpoint OK")

    # 3. Create a Key with strict RPM=3 and Quota=5000
    r = requests.post(f"{BASE_URL}/api/v1/keys", headers={
        "Authorization": f"Bearer {ADMIN_KEY}",
        "Content-Type": "application/json"
    }, json={
        "owner_id": "test_user_suite",
        "plan_id": "STARTER",
        "quota": 5000,
        "days": 1,
        "rpm_limit": 3,
        "max_concurrency": 1,
        "max_allowed_ips": 2,
        "allowed_models": "claude-opus-5.5"
    })
    assert r.status_code == 200, f"Create key failed: {r.text}"
    key_data = r.json()
    secret = key_data["key_secret"]
    key_id = key_data["key_id"]
    print(f"[PASS] 3. Virtual Key created: ID={key_id}, Quota=5000, RPM=3")

    # 4. Test Model Allowlist Violation
    r = requests.post(f"{BASE_URL}/v1/chat/completions", headers={
        "Authorization": f"Bearer {secret}",
        "Content-Type": "application/json"
    }, json={
        "model": "forbidden-model-xyz",
        "messages": [{"role": "user", "content": "hi"}]
    })
    assert r.status_code == 403, f"Expected 403 for forbidden model, got {r.status_code}"
    print("[PASS] 4. Model allowlist correctly rejected unauthorized model (HTTP 403)")

    # 5. Test Live Chat Completion via Upstream Provider (CodeCraft Opus 5.5)
    print("Sending live chat completion to upstream provider...")
    r = requests.post(f"{BASE_URL}/v1/chat/completions", headers={
        "Authorization": f"Bearer {secret}",
        "Content-Type": "application/json"
    }, json={
        "model": "claude-opus-5.5",
        "messages": [{"role": "user", "content": "Reply with 'GATEWAY_TEST_SUCCESS'"}],
        "max_tokens": 10
    }, timeout=25)
    assert r.status_code == 200, f"Chat completion failed: {r.text}"
    chat_resp = r.json()
    print(f"[PASS] 5. Live chat completion successful! Response: {chat_resp['choices'][0]['message']['content'][:60]}")

    # 6. Verify Quota Deduction in DB
    r = requests.get(f"{BASE_URL}/api/v1/keys/status?key_id={key_id}")
    status_data = r.json()
    assert status_data["remain_quota"] < 5000, f"Quota was not deducted! Remaining: {status_data['remain_quota']}"
    print(f"[PASS] 6. Atomic quota deduction verified. Remaining: {status_data['remain_quota']} (consumed: {status_data['consumed_quota']})")

    # 7. Test RPM Rate Limiter (RPM=3, so 4 rapid requests must trigger 429)
    print("Testing RPM rate limiter (sending rapid requests to trigger 429)...")
    hit_429 = False
    for i in range(5):
        r = requests.post(f"{BASE_URL}/v1/chat/completions", headers={
            "Authorization": f"Bearer {secret}",
            "Content-Type": "application/json"
        }, json={
            "model": "claude-opus-5.5",
            "messages": [{"role": "user", "content": "hi"}]
        })
        if r.status_code == 429:
            hit_429 = True
            retry_after = r.headers.get("Retry-After")
            print(f"[PASS] 7. RPM Rate Limiter successfully enforced HTTP 429! Retry-After: {retry_after}s")
            break
    assert hit_429, "RPM rate limiter failed to trigger 429 on excess requests!"

    # 8. Test Key Regeneration (Atomic Revoke & Preserved Quota/Limits)
    print("Testing atomic key regeneration...")
    r = requests.post(f"{BASE_URL}/api/v1/keys/regenerate", headers={
        "Authorization": f"Bearer {ADMIN_KEY}",
        "Content-Type": "application/json"
    }, json={
        "old_key_id": key_id,
        "actor": "admin_test"
    })
    assert r.status_code == 200, f"Regenerate failed: {r.text}"
    regen_data = r.json()
    new_secret = regen_data["key_secret"]
    new_key_id = regen_data["key_id"]
    assert new_key_id != key_id
    assert regen_data["remain_quota"] == status_data["remain_quota"], "Quota was not preserved across regeneration!"
    print(f"[PASS] 8. Key successfully regenerated: Old={key_id} -> New={new_key_id}, Quota preserved: {regen_data['remain_quota']}")

    # 9. Verify Old Key is Revoked (HTTP 401)
    r = requests.post(f"{BASE_URL}/v1/chat/completions", headers={
        "Authorization": f"Bearer {secret}",
        "Content-Type": "application/json"
    }, json={"model": "claude-opus-5.5", "messages": [{"role": "user", "content": "hi"}]})
    assert r.status_code == 401, f"Expected 401 for revoked key, got {r.status_code}"
    print("[PASS] 9. Old key is immediately revoked and blocked with HTTP 401")

    # 10. Test Coupon Redemption
    print("Testing coupon creation and redemption...")
    unique_code = f"GIFT_{int(time.time())}"
    unique_user = f"user_{int(time.time())}"
    import sqlite3
    conn = sqlite3.connect("/root/ai-key-linker/gateway.db")
    c = conn.cursor()
    c.execute("INSERT OR REPLACE INTO coupons (code, quota, days, max_uses, used_count, is_active) VALUES (?, 1000, 5, 2, 0, 1)", (unique_code,))
    conn.commit()
    conn.close()

    r = requests.post(f"{BASE_URL}/api/v1/coupons/redeem", json={
        "code": unique_code,
        "user_id": unique_user,
        "key_id": new_key_id
    })
    assert r.status_code == 200, f"Coupon redemption failed: {r.text}"
    print(f"[PASS] 10. Coupon redeemed successfully: {r.json()}")

    # Double redemption must fail
    r2 = requests.post(f"{BASE_URL}/api/v1/coupons/redeem", json={
        "code": unique_code,
        "user_id": unique_user,
        "key_id": new_key_id
    })
    assert r2.status_code == 400, "Double redemption was not prevented!"
    print("[PASS] 11. Anti-abuse: Double coupon redemption prevented")

    print("\nALL 11 GATEWAY INTEGRATION TESTS PASSED PERFECTLY!")

if __name__ == "__main__":
    test_full_suite()
