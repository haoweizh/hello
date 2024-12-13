// ucli_ffi.c

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "ucli_ffi.h"


// 模拟内部数据结构
typedef struct {
    char *buffer;
    size_t size;
} PublisherInner;

typedef struct {
    char buffer[1024];
    size_t size;
} ReceiverInner;

// 初始化市场发布者
struct UltraMarketPublisher* init_market_publisher(const char *topic,unsigned int len) {
    struct UltraMarketPublisher *publisher = malloc(sizeof(struct UltraMarketPublisher));
    if (!publisher) return NULL;

    publisher->topic = strdup(topic);
    PublisherInner *inner = malloc(sizeof(PublisherInner));
    if (!inner) {
        free(publisher);
        return NULL;
    }
    inner->buffer = NULL;
    inner->size = 0;
    publisher->inner = inner;
    printf("init_market_publisher!\n");
    return publisher;
}

// 初始化订单发布者
struct UltraOrderPublisher* init_order_publisher(const char *topic,unsigned int len) {
    struct UltraOrderPublisher *publisher = malloc(sizeof(struct UltraOrderPublisher));
    if (!publisher) return NULL;

    publisher->topic = strdup(topic);
    PublisherInner *inner = malloc(sizeof(PublisherInner));
    if (!inner) {
        free(publisher);
        return NULL;
    }
    inner->buffer = NULL;
    inner->size = 0;
    publisher->inner = inner;
    printf("init_order_publisher!\n");
    return publisher;
}

// 初始化市场接收者
struct UltraMarketReceiver* init_market_receiver(const char *topic,unsigned int len) {
    struct UltraMarketReceiver *receiver = malloc(sizeof(struct UltraMarketReceiver));
    if (!receiver) return NULL;
    receiver->topic = strdup(topic);
    ReceiverInner *inner = malloc(sizeof(ReceiverInner));
    if (!inner) {
        free(receiver);
        return NULL;
    }
    inner->buffer[0] = '\0';
    inner->size = 0;
    receiver->inner = inner;
    printf("init_market_receiver!\n");
    return receiver;
}

// 初始化订单接收者
struct UltraOrderReceiver* init_order_receiver(const char *topic,unsigned int len) {
    struct UltraOrderReceiver *receiver = malloc(sizeof(struct UltraOrderReceiver));
    if (!receiver) return NULL;

    receiver->topic = strdup(topic);
    ReceiverInner *inner = malloc(sizeof(ReceiverInner));
    if (!inner) {
        free(receiver);
        return NULL;
    }
    inner->buffer[0] = '\0';
    inner->size = 0;
    receiver->inner = inner;
    printf("init_order_receiver!\n");
    return receiver;
}

// 发布市场消息
void publish_market(struct UltraMarketPublisher *publisher, const char *msg,unsigned int len) {
    if (!publisher || !msg) return;
    printf("publish_market length: %d msg:%s \n", len,msg);
    PublisherInner *inner = (PublisherInner *)publisher->inner;
    if (inner->buffer) free(inner->buffer);
    inner->buffer = strdup(msg);
    inner->size = strlen(msg);
}

// 发布订单消息
void publish_order(struct UltraOrderPublisher *publisher, const char *msg,unsigned int len) {
    if (!publisher || !msg) return;
    printf("publish_order length: %d msg:%s \n", len,msg);
    PublisherInner *inner = (PublisherInner *)publisher->inner;
    if (inner->buffer) free(inner->buffer);
    inner->buffer = strdup(msg);
    inner->size = strlen(msg);
}

// 接收市场消息
unsigned int receive_market(struct UltraMarketReceiver *receiver, char *buf) {
    if (!receiver || !buf) {
        printf("Error: market receiver or buf is NULL\n");
        return 0;
    }
     // 假设这里是从某个地方接收数据
       const char *data = "Receive Market Data";
       size_t data_len = strlen(data);

       if (data_len >= 1024) {
           printf("Error: Buffer too small to hold the data\n");
           return 0;
       }
       memcpy(buf, data, data_len);
       buf[data_len] = '\0'; // 确保字符串以 null 结尾

       printf("Market Data received: %s\n", buf);
       printf("Market Data length: %zu\n", data_len);
      return strlen(buf);
}

// 接收订单消息
unsigned int receive_order(struct UltraOrderReceiver *receiver, char *buf) {
    if (!receiver || !buf){
         printf("Error: order receiver or buf is NULL\n");
         return 0;
    }
    // 假设这里是从某个地方接收数据
    const char *data = "Receive Order Data";
    size_t data_len = strlen(data);

    if (data_len >= 1024) {
        printf("Error: Buffer too small to hold the data\n");
        return 0;
    }
    memcpy(buf, data, data_len);
    buf[data_len] = '\0'; // 确保字符串以 null 结尾

    printf("Order Data received: %s\n", buf);
    printf("Order Data length: %zu\n", data_len);
    return strlen(buf);
}
