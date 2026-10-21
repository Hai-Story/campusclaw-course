import unittest
from support import assert_citations_equal

class CitationComparisonTests(unittest.TestCase):
    def test_mysql_one_ulp_score_rounding_is_accepted(self):
        a={'material_id':1,'excerpt':'原文','keyword_score':0.42394372820854187}
        assert_citations_equal([a],[dict(a,keyword_score=0.4239437282085418)])
    def test_evidence_scope_and_real_score_changes_are_rejected(self):
        a={'material_id':1,'excerpt':'原文','start_offset':0,'keyword_score':.4}
        for change in ({'material_id':2},{'excerpt':'不同原文'},{'start_offset':1},{'keyword_score':.401}):
            with self.subTest(change=change),self.assertRaises(AssertionError):
                assert_citations_equal([a],[dict(a,**change)])
        with self.assertRaises(AssertionError):assert_citations_equal([a],[])
        with self.assertRaises(AssertionError):assert_citations_equal([a],[dict(a,extra=1)])

if __name__=='__main__':unittest.main()
