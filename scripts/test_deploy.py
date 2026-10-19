import unittest
from deploy import valid_bucket
class WizardTests(unittest.TestCase):
 def test_reject_invalid_bucket(self):
  for v in ['','UPPER','a','bad..name','bad_name']:self.assertFalse(valid_bucket(v))
 def test_accept_account_scoped_name(self):self.assertTrue(valid_bucket('abi-pulumi-123456789012-us-east-1'))
