#!/usr/local/bin/python3

import boto3
from faker import Faker
import uuid 
import datetime
import time
import random

USERS = 10
LISTS_PER_USER = 3 
ITEMS_PER_LIST = 3
GUESTS_PER_USER = 3

WRU_UPLOAD = 2 
WRU_DEFAULT = 2

faker = Faker()
session = boto3.Session(profile_name='dynamodb_profile')
client = session.client('dynamodb')
resource = session.resource('dynamodb')


print('*** Looking for tables ***\n')
for table in ['users','guests','lists','items']:
  print('{}'.format(table),end='')
  try: 
    details = client.describe_table(TableName=table)
    # print(details['Table'])
    for n in details['Table']['KeySchema']:
      if n['KeyType'] == 'HASH':
        print('|Partition Key: {}'.format(n['AttributeName']),end='')
      if n['KeyType'] == 'RANGE':
        print('|Sort Key: {}'.format(n['AttributeName']),end='')
    print('|Item Count: {}'.format(details['Table']['ItemCount']))
    if 'GlobalSecondaryIndexes' in details['Table']:
      for gs in details['Table']['GlobalSecondaryIndexes']:
        print('{}|GSI|{}'.format(table,gs['IndexName']))
  except client.exceptions.ResourceNotFoundException as e:
    print('Error: Table {} not found'.format(table))
    print('Note: this scripts assumes tables are pre-created using terraform')
  except Exception as e:
    raise e
  
users_table = resource.Table('users')
lists_table = resource.Table('lists')
items_table = resource.Table('items')
guests_table = resource.Table('guests')

print('\n*** Increasing table capacity ***\n')

for t in [users_table,lists_table,items_table,guests_table]:
  if (t.provisioned_throughput['WriteCapacityUnits'] != WRU_UPLOAD):
    t.update(
        ProvisionedThroughput={
            'ReadCapacityUnits' : t.provisioned_throughput['ReadCapacityUnits'],
            'WriteCapacityUnits': WRU_UPLOAD 
        }
    )
    for i in range(10):
      status = t.table_status
      print('{}|{}|{}'.format(t.name,str(i),status))
      if status == "ACTIVE":
        break
      time.sleep(5)
      if i == 9:
        raise Exception('Waiting time expired') 
      t.load()
  else:
    print('{}|WRU={}|Not changed'.format(t.name,t.provisioned_throughput['WriteCapacityUnits']))

print('\n*** Populating tables ***\n')

item_counter = 0
current_time = time.time()

with users_table.batch_writer() as users_batch:
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
          item_counter = item_counter + 1
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
            item_counter = item_counter + 1
            print('list|{}'.format(list_item))

            last_item_datetime = datetime.datetime.now().isoformat()
            item_order = 0
            if len(guests) == GUESTS_PER_USER:
              for g in range(LISTS_PER_USER):
                guest_item = {
                  'list_id' : list_id, 
                  'user_id' : guests[g]
                }
                print('guest|{}'.format(guest_item))
                guests_batch.put_item(Item=guest_item)
                item_counter = item_counter + 1
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
              item_counter = item_counter + 1
          guests.append(user_id)


print('\n*** Data upload completed ***\n')

time_taken = time.time()-current_time

print('Data upload time taken: {} seconds'.format(str(time_taken)))
print('Items inserted: {} items'.format(str(item_counter)))
print('Performance: {} items/second'.format(str(item_counter / time_taken)))

print('\n*** Restoring table capacity ***\n')

for t in [users_table,lists_table,items_table,guests_table]:
  t.load()
  if (t.provisioned_throughput['WriteCapacityUnits'] != WRU_DEFAULT):
    t.update(
        ProvisionedThroughput={
            'ReadCapacityUnits' : t.provisioned_throughput['ReadCapacityUnits'],
            'WriteCapacityUnits': WRU_DEFAULT
        }
    )
    for i in range(10):
      status = t.table_status
      print('{}|{}|{}'.format(t.name,str(i),status))
      if status == "ACTIVE":
        break
      time.sleep(5)
      if i == 9:
        raise Exception('Waiting time expired') 
      t.load()
  else:
    print('{}|WRU={}|Not changed'.format(t.name,t.provisioned_throughput['WriteCapacityUnits']))

