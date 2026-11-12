#include <IOKit/hid/IOHIDManager.h>
#include <IOKit/hid/IOHIDLib.h>
#include <IOKit/hidsystem/IOHIDLib.h>
#include <ApplicationServices/ApplicationServices.h>
#include "native_darwin.h"

// The listener is owned by a single, locked Go OS thread and its CFRunLoop.
static _Thread_local IOHIDManagerRef manager;
static _Thread_local ks_key queue[256];
static _Thread_local int count,lost;
static void value(void *context,IOReturn result,void *sender,IOHIDValueRef v){
 IOHIDElementRef element=IOHIDValueGetElement(v);
 if(IOHIDElementGetUsagePage(element)!=7)return;
 int usage=IOHIDElementGetUsage(element);
 if(!((usage>=0x68&&usage<=0x73)||(usage>=0xe0&&usage<=0xe7)))return;
 if(count>=256){lost=1;return;}
 queue[count++]=(ks_key){(uint64_t)(uintptr_t)IOHIDElementGetDevice(element),usage,IOHIDValueGetIntegerValue(v)!=0};
}
int ks_input_open(void){
 if(IOHIDCheckAccess(kIOHIDRequestTypeListenEvent)!=kIOHIDAccessTypeGranted){IOHIDRequestAccess(kIOHIDRequestTypeListenEvent);return -2;}
 manager=IOHIDManagerCreate(kCFAllocatorDefault,kIOHIDOptionsTypeNone);if(!manager)return -1;
 int vid=0x1189,pid=0x8890;CFNumberRef v=CFNumberCreate(NULL,kCFNumberIntType,&vid),p=CFNumberCreate(NULL,kCFNumberIntType,&pid);
 const void *keys[]={CFSTR(kIOHIDVendorIDKey),CFSTR(kIOHIDProductIDKey)};const void *values[]={v,p};CFDictionaryRef match=CFDictionaryCreate(NULL,keys,values,2,&kCFTypeDictionaryKeyCallBacks,&kCFTypeDictionaryValueCallBacks);
 IOHIDManagerSetDeviceMatching(manager,match);CFRelease(match);CFRelease(v);CFRelease(p);
 IOHIDManagerRegisterInputValueCallback(manager,value,NULL);IOHIDManagerScheduleWithRunLoop(manager,CFRunLoopGetCurrent(),kCFRunLoopDefaultMode);count=lost=0;
 if(IOHIDManagerOpen(manager,kIOHIDOptionsTypeNone)!=kIOReturnSuccess){ks_input_close();return -1;}return 0;
}
int ks_input_read(ks_key *out,int capacity){
 CFRunLoopRunInMode(kCFRunLoopDefaultMode,0.05,true);
 if(lost){count=lost=0;return -3;}
 int n=count<capacity?count:capacity;for(int i=0;i<n;i++)out[i]=queue[i];for(int i=n;i<count;i++)queue[i-n]=queue[i];count-=n;return n;
}
void ks_input_close(void){if(manager){IOHIDManagerUnscheduleFromRunLoop(manager,CFRunLoopGetCurrent(),kCFRunLoopDefaultMode);IOHIDManagerClose(manager,0);CFRelease(manager);manager=NULL;}}
