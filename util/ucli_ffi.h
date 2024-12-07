#include <stdio.h>
#ifndef UCLI_FFI_H
#define UCLI_FFI_H
struct UltraMarketPublisher {
    void* topic;
    void* inner;
};

struct UltraOrderPublisher {
    void* topic;
    void* inner;
};

struct UltraMarketReceiver {
    void* topic;
    void* inner;
};

struct UltraOrderReceiver {
    void* topic;
    void* inner;
};



void init_market_publisher(char* topic, struct UltraMarketPublisher* publisher){
 printf("Hello from C!\n");
 }
void init_order_publisher(char* topic, struct UltraOrderPublisher* publisher){
                                                                              printf("Hello from C!\n");
                                                                              }
void init_market_receiver(char* topic, struct UltraMarketReceiver* receiver){
                                                                             printf("Hello from C!\n");
                                                                             }
void init_order_receiver(char* topic, struct UltraOrderReceiver* receiver){
                                                                           printf("Hello from C!\n");
                                                                           }

void publish_market(struct UltraMarketPublisher* publisher, char* msg){
                                                                       printf("Hello from C!\n");
                                                                       }
void publish_order(struct UltraOrderPublisher* publisher, char* msg){
                                                                     printf("Hello from C!\n");
                                                                     }
void receive_market(struct UltraMarketReceiver* receiver, char* buf){
                                                                     printf("Hello from C!\n");
                                                                     }
void receive_order(struct UltraOrderReceiver* receiver, char* buf){
                                                                   printf("Hello from C!\n");
                                                                   }

#endif //UCLI_FFI_H
