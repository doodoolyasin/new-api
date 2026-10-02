import httpx
from typing import Optional, Dict, Any, List
from config import GATEWAY_URL, ADMIN_KEY

class GatewayClient:
    def __init__(self, base_url: str = GATEWAY_URL, admin_key: str = ADMIN_KEY):
        self.base_url = base_url.rstrip("/")
        self.admin_key = admin_key
        self.headers = {
            "Authorization": f"Bearer {self.admin_key}",
            "Content-Type": "application/json"
        }

    async def create_key(
        self,
        owner_id: str,
        plan_id: str,
        quota: int,
        days: int,
        rpm_limit: int,
        max_concurrency: int = 1,
        max_allowed_ips: int = 2,
        allowed_models: str = "*",
        note: str = ""
    ) -> Optional[Dict[str, Any]]:
        url = f"{self.base_url}/api/v1/keys"
        payload = {
            "owner_id": str(owner_id),
            "plan_id": plan_id,
            "quota": quota,
            "days": days,
            "rpm_limit": rpm_limit,
            "max_concurrency": max_concurrency,
            "max_allowed_ips": max_allowed_ips,
            "allowed_models": allowed_models,
            "note": note
        }
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.post(url, headers=self.headers, json=payload)
            if resp.status_code == 200:
                return resp.json()
            return None

    async def get_user_keys(self, owner_id: str) -> List[Dict[str, Any]]:
        url = f"{self.base_url}/api/v1/keys?owner_id={owner_id}"
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.get(url, headers=self.headers)
            if resp.status_code == 200:
                return resp.json() or []
            return []

    async def get_key_status(self, key_id: str) -> Optional[Dict[str, Any]]:
        url = f"{self.base_url}/api/v1/keys/status?key_id={key_id}"
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.get(url)
            if resp.status_code == 200:
                return resp.json()
            return None

    async def regenerate_key(self, old_key_id: str, actor: str) -> Optional[Dict[str, Any]]:
        url = f"{self.base_url}/api/v1/keys/regenerate"
        payload = {
            "old_key_id": old_key_id,
            "actor": actor
        }
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.post(url, headers=self.headers, json=payload)
            if resp.status_code == 200:
                return resp.json()
            return None

    async def revoke_key(self, key_id: str, actor: str) -> bool:
        url = f"{self.base_url}/api/v1/keys/revoke"
        payload = {
            "key_id": key_id,
            "actor": actor
        }
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.post(url, headers=self.headers, json=payload)
            return resp.status_code == 200

    async def add_quota(self, key_id: str, amount: int, tx_type: str = "PURCHASE", details: str = "", actor: str = "bot") -> Optional[int]:
        url = f"{self.base_url}/api/v1/keys/quota"
        payload = {
            "key_id": key_id,
            "amount": amount,
            "type": tx_type,
            "details": details,
            "actor": actor
        }
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.post(url, headers=self.headers, json=payload)
            if resp.status_code == 200:
                return resp.json().get("new_balance")
            return None

    async def redeem_coupon(self, code: str, user_id: str, key_id: str) -> Optional[Dict[str, Any]]:
        url = f"{self.base_url}/api/v1/coupons/redeem"
        payload = {
            "code": code.strip().upper(),
            "user_id": str(user_id),
            "key_id": key_id
        }
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.post(url, json=payload)
            if resp.status_code == 200:
                return resp.json()
            return None

    async def get_admin_stats(self) -> Optional[Dict[str, Any]]:
        url = f"{self.base_url}/api/v1/admin/stats"
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.get(url, headers=self.headers)
            if resp.status_code == 200:
                return resp.json()
            return None

gateway_client = GatewayClient()
