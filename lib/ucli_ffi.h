//
// Created by c on 12/9/24.
//

#ifndef UCLI_FFI_H
#define UCLI_FFI_H

struct UltraMarketPublisher {
    const void *topic;
    const void *inner;
};

struct UltraOrderPublisher {
    void *topic;
    void *inner;
};

struct UltraMarketReceiver {
    void *topic;
    void *inner;
};

struct UltraOrderReceiver {
    void *topic;
    void *inner;
};

extern struct UltraMarketPublisher* init_market_publisher(const char *topic,unsigned int len);

extern struct UltraOrderPublisher* init_order_publisher(const char *topic,unsigned int len);

extern struct UltraMarketReceiver* init_market_receiver(const char *topic,unsigned int len);

extern struct UltraOrderReceiver* init_order_receiver(const char *topic,unsigned int len);

extern void publish_market(struct UltraMarketPublisher *publisher, const char *msg,unsigned int len);

extern void publish_order(struct UltraOrderPublisher *publisher, const char *msg,unsigned int len);

extern unsigned int receive_market(struct UltraMarketReceiver *receiver, char *buf);

extern unsigned int receive_order(struct UltraOrderReceiver *receiver, char *buf);

#endif //UCLI_FFI_H
