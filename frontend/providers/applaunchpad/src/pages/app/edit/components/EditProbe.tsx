import {
  Box,
  Button,
  FormControl,
  FormLabel,
  FormErrorMessage,
  HStack,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  NumberInput,
  NumberInputField,
  Select,
  Stack,
  Switch,
  Text,
  Textarea,
  useDisclosure,
  VStack
} from '@chakra-ui/react';
import { useTranslation } from 'next-i18next';
import { ProbeType } from '@/types/app';
import React, { useEffect, useState } from 'react';
import MyIcon from '@/components/Icon';

interface EditProbeProps {
  probeType: 'livenessProbe' | 'readinessProbe' | 'startupProbe';
  defaultProbe?: ProbeType;
  onSuccess: (data: ProbeType) => void;
}

const EditProbe: React.FC<EditProbeProps> = ({ probeType, defaultProbe, onSuccess }) => {
  const { t } = useTranslation();
  const { isOpen, onOpen, onClose } = useDisclosure();
  const [probe, setProbe] = useState<ProbeType>(defaultProbe || { use: false });
  const [errors, setErrors] = useState<{ [key: string]: string }>({});

  useEffect(() => {
    setProbe(defaultProbe || { use: false });
  }, [defaultProbe]);

  const validate = () => {
    const newErrors: { [key: string]: string } = {};

    if (probe.use) {
      if (probe.initialDelaySeconds !== undefined && probe.initialDelaySeconds < 0) {
        newErrors.initialDelaySeconds = t('Value must be greater than or equal to 0');
      }
      if (probe.periodSeconds !== undefined && probe.periodSeconds < 0) {
        newErrors.periodSeconds = t('Value must be greater than or equal to 0');
      }
      if (probe.timeoutSeconds !== undefined && probe.timeoutSeconds < 0) {
        newErrors.timeoutSeconds = t('Value must be greater than or equal to 0');
      }
      if (probe.successThreshold !== undefined && probe.successThreshold < 0) {
        newErrors.successThreshold = t('Value must be greater than or equal to 0');
      }
      if (probe.failureThreshold !== undefined && probe.failureThreshold < 0) {
        newErrors.failureThreshold = t('Value must be greater than or equal to 0');
      }
      if (probe.httpGet?.port !== undefined && probe.httpGet.port < 0) {
        newErrors.httpGetPort = t('Value must be greater than or equal to 0');
      }
      if (probe.tcpSocket?.port !== undefined && probe.tcpSocket.port < 0) {
        newErrors.tcpSocketPort = t('Value must be greater than or equal to 0');
      }
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSave = () => {
    if (validate()) {
      onSuccess(probe);
      onClose();
    }
  };

  const handleInputChange = (
    field: keyof Omit<ProbeType, 'use' | 'exec' | 'httpGet' | 'tcpSocket'>,
    value: string | number
  ) => {
    setProbe((prevProbe) => ({
      ...prevProbe,
      [field]: value
    }));
  };

  const handleExecChange = (command: string[]) => {
    setProbe((prevProbe) => ({
      ...prevProbe,
      exec: { command },
      httpGet: undefined,
      tcpSocket: undefined
    }));
  };

  const handleHttpGetChange = (
    field: keyof NonNullable<ProbeType['httpGet']>,
    value: string | number
  ) => {
    setProbe((prevProbe) => ({
      ...prevProbe,
      httpGet: {
        path: prevProbe.httpGet?.path || '',
        port: prevProbe.httpGet?.port || 0,
        [field]: value
      },
      exec: undefined,
      tcpSocket: undefined
    }));
  };

  const handleTcpSocketChange = (port: number) => {
    setProbe((prevProbe) => ({
      ...prevProbe,
      tcpSocket: { port },
      exec: undefined,
      httpGet: undefined
    }));
  };

  return (
    <>
      <Button
        w={'100%'}
        variant={'outline'}
        fontSize={'base'}
        leftIcon={<MyIcon name="edit" width={'16px'} fill={'#485264'} />}
        onClick={onOpen}
      >
        {t(`Edit ${probeType}`)}
      </Button>

      <Modal isOpen={isOpen} onClose={onClose}>
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>{t(`Edit ${probeType}`)}</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <VStack spacing={4} align="start">
              <FormControl>
                <FormLabel>{t('Enable Probe')}</FormLabel>
                <Switch
                  isChecked={probe.use}
                  onChange={(e) =>
                    setProbe((prevProbe) => ({ ...prevProbe, use: e.target.checked }))
                  }
                />
              </FormControl>
              {probe.use && (
                <VStack spacing={4} align="start">
                  <FormControl isInvalid={!!errors.initialDelaySeconds}>
                    <FormLabel>{t('initialDelaySeconds')}</FormLabel>
                    <NumberInput
                      value={probe.initialDelaySeconds || ''}
                      onChange={(valueString) =>
                        handleInputChange('initialDelaySeconds', Number(valueString))
                      }
                      min={0}
                    >
                      <NumberInputField />
                    </NumberInput>
                    {errors.initialDelaySeconds && (
                      <FormErrorMessage>{errors.initialDelaySeconds}</FormErrorMessage>
                    )}
                  </FormControl>
                  <FormControl isInvalid={!!errors.periodSeconds}>
                    <FormLabel>{t('periodSeconds')}</FormLabel>
                    <NumberInput
                      value={probe.periodSeconds || ''}
                      onChange={(valueString) =>
                        handleInputChange('periodSeconds', Number(valueString))
                      }
                      min={0}
                    >
                      <NumberInputField />
                    </NumberInput>
                    {errors.periodSeconds && (
                      <FormErrorMessage>{errors.periodSeconds}</FormErrorMessage>
                    )}
                  </FormControl>
                  <FormControl isInvalid={!!errors.timeoutSeconds}>
                    <FormLabel>{t('timeoutSeconds')}</FormLabel>
                    <NumberInput
                      value={probe.timeoutSeconds || ''}
                      onChange={(valueString) =>
                        handleInputChange('timeoutSeconds', Number(valueString))
                      }
                      min={0}
                    >
                      <NumberInputField />
                    </NumberInput>
                    {errors.timeoutSeconds && (
                      <FormErrorMessage>{errors.timeoutSeconds}</FormErrorMessage>
                    )}
                  </FormControl>
                  <FormControl isInvalid={!!errors.successThreshold}>
                    <FormLabel>{t('successThreshold')}</FormLabel>
                    <NumberInput
                      value={probe.successThreshold || ''}
                      onChange={(valueString) =>
                        handleInputChange('successThreshold', Number(valueString))
                      }
                      min={0}
                    >
                      <NumberInputField />
                    </NumberInput>
                    {errors.successThreshold && (
                      <FormErrorMessage>{errors.successThreshold}</FormErrorMessage>
                    )}
                  </FormControl>
                  <FormControl isInvalid={!!errors.failureThreshold}>
                    <FormLabel>{t('failureThreshold')}</FormLabel>
                    <NumberInput
                      value={probe.failureThreshold || ''}
                      onChange={(valueString) =>
                        handleInputChange('failureThreshold', Number(valueString))
                      }
                      min={0}
                    >
                      <NumberInputField />
                    </NumberInput>
                    {errors.failureThreshold && (
                      <FormErrorMessage>{errors.failureThreshold}</FormErrorMessage>
                    )}
                  </FormControl>
                  <FormControl as={Stack} spacing={4}>
                    <FormLabel>{t('Probe Type')}</FormLabel>
                    <Select
                      value={
                        probe.exec
                          ? 'exec'
                          : probe.httpGet
                          ? 'httpGet'
                          : probe.tcpSocket
                          ? 'tcpSocket'
                          : ''
                      }
                      onChange={(e) => {
                        if (
                          e.target.value === 'exec' ||
                          e.target.value === 'httpGet' ||
                          e.target.value === 'tcpSocket'
                        ) {
                          setProbe((prevProbe) => ({
                            ...prevProbe,
                            exec: e.target.value === 'exec' ? { command: [] } : undefined,
                            httpGet:
                              e.target.value === 'httpGet' ? { path: '/', port: 80 } : undefined,
                            tcpSocket: e.target.value === 'tcpSocket' ? { port: 80 } : undefined
                          }));
                          return;
                        }
                        setProbe((prevProbe) => ({
                          ...prevProbe,
                          exec: undefined,
                          httpGet: undefined,
                          tcpSocket: undefined
                        }));
                      }}
                    >
                      <option value="">{t('None')}</option>
                      <option value="exec">{t('Exec')}</option>
                      <option value="httpGet">{t('HTTP Get')}</option>
                      <option value="tcpSocket">{t('TCP Socket')}</option>
                    </Select>
                    {probe.exec && (
                      <FormControl>
                        <FormLabel>{t('Command')}</FormLabel>
                        <Textarea
                          value={probe.exec.command.join(' ')}
                          onChange={(e) => handleExecChange(e.target.value.split(' '))}
                        />
                      </FormControl>
                    )}
                    {probe.httpGet && (
                      <VStack spacing={4} align="start">
                        <FormControl>
                          <FormLabel>{t('Path')}</FormLabel>
                          <Input
                            value={probe.httpGet.path}
                            onChange={(e) => handleHttpGetChange('path', e.target.value)}
                          />
                        </FormControl>
                        <FormControl isInvalid={!!errors.httpGetPort}>
                          <FormLabel>{t('Port')}</FormLabel>
                          <NumberInput
                            value={probe.httpGet.port}
                            onChange={(valueString) =>
                              handleHttpGetChange('port', Number(valueString))
                            }
                            min={0}
                          >
                            <NumberInputField />
                          </NumberInput>
                          {errors.httpGetPort && (
                            <FormErrorMessage>{errors.httpGetPort}</FormErrorMessage>
                          )}
                        </FormControl>
                        {/* Add other httpGet fields here */}
                      </VStack>
                    )}
                    {probe.tcpSocket && (
                      <FormControl isInvalid={!!errors.tcpSocketPort}>
                        <FormLabel>{t('Port')}</FormLabel>
                        <NumberInput
                          value={probe.tcpSocket.port}
                          onChange={(valueString) => handleTcpSocketChange(Number(valueString))}
                          min={0}
                        >
                          <NumberInputField />
                        </NumberInput>
                        {errors.tcpSocketPort && (
                          <FormErrorMessage>{errors.tcpSocketPort}</FormErrorMessage>
                        )}
                      </FormControl>
                    )}
                  </FormControl>
                </VStack>
              )}
            </VStack>
          </ModalBody>
          <ModalFooter>
            <Button variant="ghost" onClick={onClose}>
              {t('Cancel')}
            </Button>
            <Button colorScheme="blue" ml={3} onClick={handleSave}>
              {t('Save')}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </>
  );
};

export default EditProbe;
