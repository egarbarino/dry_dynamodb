#!/usr/local/bin/python3

import boto3
from faker import Faker
import uuid 
import datetime
import random

USERS = 10
LISTS_PER_USER = 3 
ITEMS_PER_LIST = 3
GUESTS_PER_USER = 3

faker = Faker()
session = boto3.Session(profile_name='dynamodb_profile')
client = session.client('dynamodb')
resource = session.resource('dynamodb')


# table_users = db_r.Table('users_x')
# print(table_users.creation_date_time)

print('Looking for tables...\n')
for table in ['users','guests','lists','items']:
  print('Table: {}'.format(table))
  try: 
    details = client.describe_table(TableName=table)
    # print(details['Table'])
    for n in details['Table']['KeySchema']:
      if n['KeyType'] == 'HASH':
        print('  Partition Key: {}'.format(n['AttributeName']))
      if n['KeyType'] == 'RANGE':
        print('  Sort Key: {}'.format(n['AttributeName']))
    print('  Item Count: {}'.format(details['Table']['ItemCount']))
    if 'GlobalSecondaryIndexes' in details['Table']:
      print('  Global Secondary Indexes:')
      for gs in details['Table']['GlobalSecondaryIndexes']:
        print('    - {}'.format(gs['IndexName']))
  except client.exceptions.ResourceNotFoundException as e:
    print('  Error: Table {} not found'.format(table))
    print('  Note: this scripts assumes tables are pre-created using terraform')
  except Exception as e:
    raise e

with resource.Table('users').batch_writer() as users_batch:
  with resource.Table('lists').batch_writer() as lists_batch:
    with resource.Table('items').batch_writer() as items_batch:
      with resource.Table('guests').batch_writer() as guests_batch:

        guests = []
        for i in range(USERS):
          user_id = str(uuid.uuid4())
          user_email = faker.email()
          user_item = {
            'id' : user_id, 
            'email' : user_email
          } 
          users_batch.put_item(Item=user_item) 
          print('user|{}'.format(user_item))

          for n in range(LISTS_PER_USER):
            list_id = str(uuid.uuid4())
            list_title = faker.sentence() 
            list_item = {
              'id' : list_id, 
              'user_id' : user_id, 
              'title' : list_title
            } 
            lists_batch.put_item(Item=list_item)
            print('list|{}'.format(list_item))

            last_item_datetime = datetime.datetime.now().isoformat()
            item_order = 0
            print(guests)
            if len(guests) == GUESTS_PER_USER:
              for g in range(LISTS_PER_USER):
                guest_item = {
                  'list_id' : list_id, 
                  'user_id' : guests[g]
                }
                print('guest|{}'.format(guest_item))
                guests_batch.put_item(Item=guest_item)
              guests = []

            for o in range(ITEMS_PER_LIST):
              item_datetime = datetime.datetime.now().isoformat()
              item_description = faker.sentence()
              item_done = random.choice([True,False])
              item_order = item_order + 1
              item_item = {
                'list_id' : list_id, 
                'datetime' : item_datetime, 
                'description' : item_description,
                'done' : item_done, 
                'order' : item_order}
              print('item|{}'.format(item_item))

              while last_item_datetime == item_datetime:
                item_datetime = datetime.datetime.now().isoformat()
              last_item_datetime = item_datetime
              items_batch.put_item(Item=item_item)

          guests.append(user_id)


# batch.put_item(Item=item_gen(Type=Table, uuid=uuiddict[Table][c],uuiddict=uuiddict, Idbucket=idbucket))
# x = db_c.describe_table(TableName='users')
# print(x)