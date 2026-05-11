import unittest

from _path import SRC  # noqa: F401
from living_memoryv2.mobius import MobiusAddress, MobiusIndex
from living_memoryv2.schema import MemoryEnvelope


class MobiusIndexTests(unittest.TestCase):
    def test_address_for_memory_is_deterministic(self):
        index = MobiusIndex(u_size=32, v_size=16, w_size=8)
        memory = MemoryEnvelope.text(
            "Token refresh solution",
            scope={"project_id": "psi"},
            memory_type="solution",
            tags=["oauth"],
            entities=["refresh_token"],
            created_at=1.0,
        )

        self.assertEqual(index.address_for(memory), index.address_for(memory))
        self.assertEqual(index.address_for(memory).region, "solution")

    def test_neighbors_wrap_mobius_u_axis_and_stay_bounded(self):
        index = MobiusIndex(u_size=8, v_size=4, w_size=3)
        address = MobiusAddress(iu=0, iv=1, iw=1, region="episodic")

        neighbors = index.neighbors(address, depth=1)

        self.assertIn(MobiusAddress(iu=7, iv=2, iw=1, region="episodic"), neighbors)
        self.assertTrue(all(0 <= item.iu < 8 for item in neighbors))
        self.assertTrue(all(0 <= item.iv < 4 for item in neighbors))
        self.assertTrue(all(0 <= item.iw < 3 for item in neighbors))

    def test_neighbors_handle_single_v_band_without_looping(self):
        index = MobiusIndex(u_size=1, v_size=1, w_size=1)
        address = MobiusAddress(iu=0, iv=0, iw=0, region="episodic")

        neighbors = index.neighbors(address, depth=1)

        self.assertEqual(neighbors, [address])

    def test_project_scope_changes_address_for_same_content(self):
        index = MobiusIndex(u_size=32, v_size=16, w_size=8)
        first = MemoryEnvelope.text("Same fact", scope={"project_id": "alpha"}, created_at=1.0)
        second = MemoryEnvelope.text("Same fact", scope={"project_id": "beta"}, created_at=1.0)

        self.assertNotEqual(index.address_for(first), index.address_for(second))


if __name__ == "__main__":
    unittest.main()
