#include "ucli_ffi.h"
#include <stdio.h>
#include <stdlib.h>

void init_market_publisher(char* topic, struct UltraMarketPublisher* publisher)
{
    printf("init_market_publisher!\n");
}
void init_order_publisher(char* topic, struct UltraOrderPublisher* publisher)
{
    printf("init_order_publisher!\n");
}
void init_market_receiver(char* topic, struct UltraMarketReceiver* receiver)
{
    printf("init_market_receiver\n");
}
void init_order_receiver(char* topic, struct UltraOrderReceiver* receiver)
{
    printf("init_order_receiver!\n");
}

void publish_market(struct UltraMarketPublisher* publisher, char* msg)
{
    printf("publish_market!\n");
}
void publish_order(struct UltraOrderPublisher* publisher, char* msg){
                                                                        printf("publish_order!\n");
                                                                    }

void receive_market(struct UltraMarketReceiver* receiver, char* buf){
                                                                        printf("receive_market!\n");
                                                                    }
void receive_order(struct UltraOrderReceiver* receiver, char* buf){
                                                                      printf("receive_order!\n");
                                                                  }

