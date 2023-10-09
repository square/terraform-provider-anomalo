# General syntax
terraform import anomalo_check.check_name table_id,check_static_id,check_ref

# In practice, only one of check_id or check_ref should be provided.
terraform import anomalo_check.check_name table_id,check_static_id
terraform import anomalo_check.check_name table_id,,check_ref # Note the double comma
