from abc import ABC, abstractmethod
from typing import Dict, Any

class LLMProvider(ABC):
    @abstractmethod
    async def generate_json(self, system_prompt: str, user_prompt: str) -> Dict[str, Any]:
        """Generate structured JSON response from LLM."""
        pass
